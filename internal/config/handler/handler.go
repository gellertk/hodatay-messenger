package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/kgellert/hodatay-messenger/internal/config"
)

func New(config config.Config, logger *slog.Logger) *Handler {
	return &Handler{config, logger}
}

type appConfigResponse struct {
	Config config.Config `json:"config"`
}

type Handler struct {
	Config config.Config
	log *slog.Logger
}

func (h *Handler) GetConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.configHandler.GetConfig"

		log := h.log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		log.Debug("config requested")

		render.JSON(w, r, appConfigResponse{
			Config:   h.Config,
		})
	}
}