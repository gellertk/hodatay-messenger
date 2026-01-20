package uploadsHandler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/kgellert/hodatay-messenger/internal/lib/api/response"
	"github.com/kgellert/hodatay-messenger/internal/lib/logger/sl"
)

func (h *UploadsHandler) PresignDownload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.presignDownload.PresignDownload"

		log := h.Log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var req presignDownloadRequest
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			log.Error("invalid body", sl.Err(err))
			render.JSON(w, r, response.Error("invalid body"))
			return
		}

		if req.Key == "" {
			log.Error("invalid key")
			render.JSON(w, r, response.Error("invalid key"))
			return
		}

		url, err := h.Storage.PresignDownload(r.Context(), req.Key)

		if err != nil {
			log.Error("presign upload error", sl.Err(err))
			render.JSON(w, r, response.Error("presign upload error"))
			return
		}

		presignResponse := presignDownloadResponse{
			url,
		}

		render.JSON(w, r, PresignDownloadHTTPResponse{
			Response: response.OK(),
			PresignDownloadResponse: presignResponse,
		})
	}
}