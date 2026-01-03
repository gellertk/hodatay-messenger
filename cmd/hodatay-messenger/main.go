package main

import (
	stdlog "log"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"

	"github.com/kgellert/hodatay-messenger/internal/config"
	"github.com/kgellert/hodatay-messenger/internal/http-server/handlers/chats"
	messagesHandler "github.com/kgellert/hodatay-messenger/internal/http-server/handlers/messages"
	mwLogger "github.com/kgellert/hodatay-messenger/internal/http-server/middleware/logger"
	"github.com/kgellert/hodatay-messenger/internal/lib/logger/handlers/slogpretty"
	"github.com/kgellert/hodatay-messenger/internal/lib/logger/sl"
	"github.com/kgellert/hodatay-messenger/internal/storage/sqlite"
	ws "github.com/kgellert/hodatay-messenger/internal/ws/handler"
	"github.com/kgellert/hodatay-messenger/internal/ws/hub"
)

const (
	envLocal = "local"
	envDev   = "dev"
)

func main() {
	if err := godotenv.Load(); err != nil {
		stdlog.Println("No .env file found, skipping...")
	}

	cfg := config.MustLoad()

	log := setupLogger(cfg.Env)
	log.Info("starting hodatay-messenger", slog.String("env", cfg.Env))
	log.Debug("debug messages are enabled")
	log.Error("error messages are enabled")

	storage, err := sqlite.New(cfg.StoragePath)
	if err != nil {
		log.Error("failed to init storage", sl.Err(err))
		os.Exit(1)
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger) // можно убрать, если используешь свой только
	router.Use(mwLogger.New(log))
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)

	router.Get("/chats", chatsHandler.GetChats(log, storage))
	router.Get("/chats/{chatID}", chatsHandler.GetChat(log, storage))

	h := hub.NewHub()
	go h.Run()

	router.Get("/ws", ws.WSHandler(h, log))

	mh := messagesHandler.New(
    storage,
    h,
    log,
	)

	router.Post("/chats/{chatID}/messages", mh.SendMessage())
	router.Patch("/chats/{chatID}/messages/read", mh.SetLastReadMessage())
	router.Get("/chats/{chatID}/messages", mh.GetMessages())

	log.Info("starting server", slog.String("address", cfg.Address))

	srv := &http.Server{
		Addr: cfg.Address,
		Handler: router,
		ReadTimeout: cfg.HTTPServer.Timeout,
		WriteTimeout: cfg.HTTPServer.Timeout,
		IdleTimeout: cfg.HTTPServer.IdleTimeout,
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
