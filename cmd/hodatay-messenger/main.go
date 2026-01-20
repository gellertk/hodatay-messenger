package main

import (
	"context"
	stdlog "log"
	"log/slog"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"

	appConfig "github.com/kgellert/hodatay-messenger/internal/config"
	"github.com/kgellert/hodatay-messenger/internal/http-server/handlers/chats"
	messagesHandler "github.com/kgellert/hodatay-messenger/internal/http-server/handlers/messages"
	uploadsHandler "github.com/kgellert/hodatay-messenger/internal/http-server/handlers/uploads"
	mwLogger "github.com/kgellert/hodatay-messenger/internal/http-server/middleware/logger"
	"github.com/kgellert/hodatay-messenger/internal/lib/logger/handlers/slogpretty"
	"github.com/kgellert/hodatay-messenger/internal/lib/logger/sl"
	"github.com/kgellert/hodatay-messenger/internal/storage/postgres"
	tempuser "github.com/kgellert/hodatay-messenger/internal/tempuser"
	"github.com/kgellert/hodatay-messenger/internal/uploads"
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

	storage, err := storage.New(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Error("failed to init storage", sl.Err(err))
		os.Exit(1)
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(mwLogger.New(log))
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

	uploadService := uploads.NewService(bucket, presigner, s3Client)

	mh := messagesHandler.New(
		storage,
		uploadService,
		h,
		log,
	)

	uh := uploadsHandler.New(
		uploadService,
		log,
	)

	router.Post("/signin", func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.Query().Get("user_id")
		if raw == "" {
			http.Error(w, "missing user_id", http.StatusBadRequest)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:  "user_id",
			Value: raw,
			Path:  "/",
		})

		w.WriteHeader(http.StatusOK)
	})

	router.Group(func(r chi.Router) {
		r.Use(tempuser.WithUser)

		r.Get("/chats", chatsHandler.GetChats(log, storage))
		r.Get("/chats/{chatId}", chatsHandler.GetChat(log, storage))

		r.Get("/ws", ws.WSHandler(h, log))

		r.Post("/chats/{chatId}/messages", mh.SendMessage())
		r.Patch("/chats/{chatId}/messages/read", mh.SetLastReadMessage())
		r.Get("/chats/{chatId}/messages", mh.GetMessages())

		r.Post("/uploads/presign-upload", uh.PresignUpload())
		r.Post("/uploads/presign-download", uh.PresignDownload())
	})

	log.Info("starting server", slog.String("address", cfg.Address))

	srv := &http.Server{
		Addr:         cfg.Address,
		Handler:      router,
		ReadTimeout:  cfg.HTTPServer.Timeout,
		WriteTimeout: cfg.HTTPServer.Timeout,
		IdleTimeout:  cfg.HTTPServer.IdleTimeout,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Error("failed to start server")
	}

	log.Error("server stopped")
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
