package main

import (
	"context"
	"fmt"
	stdlog "log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"

	chatshandler "github.com/kgellert/hodatay-messenger/internal/chats/handler"
	chatsrepo "github.com/kgellert/hodatay-messenger/internal/chats/repo"
	appConfig "github.com/kgellert/hodatay-messenger/internal/config"
	configHandler "github.com/kgellert/hodatay-messenger/internal/config/handler"
	"github.com/kgellert/hodatay-messenger/internal/logger"
	"github.com/kgellert/hodatay-messenger/internal/logger/handlers/slogpretty"
	"github.com/kgellert/hodatay-messenger/internal/logger/sl"
	messageshandler "github.com/kgellert/hodatay-messenger/internal/messages/handler"
	messagesrepo "github.com/kgellert/hodatay-messenger/internal/messages/repo"
	uploadshandler "github.com/kgellert/hodatay-messenger/internal/uploads/handler"
	uploadsrepo "github.com/kgellert/hodatay-messenger/internal/uploads/repo"
	uploadsservice "github.com/kgellert/hodatay-messenger/internal/uploads/service"
	userhandlers "github.com/kgellert/hodatay-messenger/internal/users/handlers"
	usersrepo "github.com/kgellert/hodatay-messenger/internal/users/repo"
	ws "github.com/kgellert/hodatay-messenger/internal/ws/handler"
	"github.com/kgellert/hodatay-messenger/internal/ws/hub"
)

const (
	envLocal = "local"
	envDev   = "dev"
)

func main() {
	if err := godotenv.Load("infra/.env"); err != nil {
		stdlog.Println("No .env file found, skipping...")
	}

	cfg := appConfig.MustLoad()

	log := setupLogger(cfg.Env)
	log.Info("starting hodatay-messenger", slog.String("env", cfg.Env))

	ctx := context.Background()

	log.Info("database dsn loaded", slog.String("dsn", cfg.DatabaseDSN))

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(logger.New(log))
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)

	h := hub.NewHub()
	go h.Run()

	bucket := os.Getenv("S3_BUCKET")
	region := os.Getenv("S3_REGION")
	endpoint := os.Getenv("S3_ENDPOINT")
	accessKey := os.Getenv("S3_ACCESS_KEY")
	secretKey := os.Getenv("S3_SECRET_KEY")

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
	)

	if err != nil {
		log.Error("failed to load aws config", sl.Err(err))
		os.Exit(1)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	presigner := s3.NewPresignClient(s3Client)

	db, err := initDB(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Error("failed to init storage", sl.Err(err))
		os.Exit(1)
	}

	usersRepo := usersrepo.New(db)
	chatsRepo := chatsrepo.New(db, usersRepo)
	messagesRepo := messagesrepo.New(db)
	uploadsRepo := uploadsrepo.New(db)

	uploadsService := uploadsservice.New(bucket, presigner, s3Client, uploadsRepo, cfg.Uploads.PresignTTL)

	configHandler := configHandler.New(*cfg, log)
	usersHandler := userhandlers.New(usersRepo, log)
	chatsHandler := chatshandler.New(chatsRepo, log)
	messagesHandler := messageshandler.New(
		messagesRepo,
		uploadsService,
		h,
		log,
	)
	uploadsHandler := uploadshandler.New(
		uploadsService,
		log,
	)

	router.Get("/config", configHandler.GetConfig())

	router.Post("/signin", usersHandler.SignInHandler())

	router.Group(func(r chi.Router) {
		r.Use(userhandlers.WithUser)

		r.Post("/chats", chatsHandler.CreateChat())
		r.Get("/chats", chatsHandler.GetChats())
		r.Get("/chats/{chatId}", chatsHandler.GetChat())
		r.Get("/chats/unread-count", chatsHandler.GetUnreadMessagesCount())
		r.Post("/chats/delete", chatsHandler.DeleteChats())

		r.Get("/ws", ws.WSHandler(h, log))

		r.Post("/chats/{chatId}/messages", messagesHandler.SendMessage())
		r.Patch("/chats/{chatId}/messages/read", messagesHandler.SetLastReadMessage())
		r.Get("/chats/{chatId}/messages", messagesHandler.GetMessages())

		r.Post("/uploads/presign-upload", uploadsHandler.PresignUpload())
		r.Post("/uploads/presign-download", uploadsHandler.PresignDownload())
		r.Post("/uploads/confirm", uploadsHandler.ConfirmUpload())
	})

	log.Info("starting server", slog.String("address", cfg.HTTPServer.Address))

	srv := &http.Server{
		Addr:         cfg.HTTPServer.Address,
		Handler:      router,
		ReadTimeout:  cfg.HTTPServer.Timeout,
		WriteTimeout: cfg.HTTPServer.Timeout,
		IdleTimeout:  cfg.HTTPServer.IdleTimeout,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Error("failed to start server", sl.Err(err))
		os.Exit(1)
	}
}

func initDB(ctx context.Context, dsn string) (*sqlx.DB, error) {
	const op = "storage.postgres.New"

	db, err := sqlx.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("%s: open: %w", op, err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%s: ping: %w", op, err)
	}

	return db, nil
}

func setupPrettySlog() *slog.Logger {
	opts := slogpretty.PrettyHandlerOptions{
		SlogOpts: &slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
	}

	handler := opts.NewPrettyHandler(os.Stdout)

	return slog.New(handler)
}

func setupLogger(env string) *slog.Logger {
	switch env {
	case envLocal:
		return setupPrettySlog()
	case envDev:
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	default:
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
}
