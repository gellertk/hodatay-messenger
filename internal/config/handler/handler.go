package configHandler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/kgellert/hodatay-messenger/internal/config"
	response "github.com/kgellert/hodatay-messenger/internal/lib"
)

func New(config config.AppConfig, logger *slog.Logger) *Handler {
	return &Handler{config, logger}
}

type appConfigResponse struct {
	response.Response
	Config config.AppConfig `json:"config"`
}

type Handler struct {
	appConfig config.AppConfig
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

		resp := appConfigResponse{
			Response: response.OK(),
			Config:   h.appConfig,
		}

		render.JSON(w, r, resp)
	}
}