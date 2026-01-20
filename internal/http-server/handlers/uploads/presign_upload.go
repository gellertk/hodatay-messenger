package uploadsHandler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/kgellert/hodatay-messenger/internal/lib/api/response"
	"github.com/kgellert/hodatay-messenger/internal/lib/logger/sl"
	"github.com/kgellert/hodatay-messenger/internal/uploads"
)

type UploadsHandler struct {
	Storage uploads.UploadsService
	Log     *slog.Logger
}

func New(storage uploads.UploadsService, log *slog.Logger) *UploadsHandler {
	return &UploadsHandler{
		storage,
		log,
	}
}

func (h *UploadsHandler) PresignUpload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.uploads.PresignUpload"

		log := h.Log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var req presignUploadRequest
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			log.Error("invalid body", sl.Err(err))
			render.JSON(w, r, response.Error("invalid body"))
			return
		}

		if req.ContentType == "" {
			log.Error("invalid content type")
			render.JSON(w, r, response.Error("invalid content type"))
			return
		}

		if req.Filename == "" {
			log.Error("invalid filename")
			render.JSON(w, r, response.Error("invalid filename"))
			return
		}

		key, url, err := h.Storage.PresignUpload(r.Context(), req.Filename, req.ContentType)

		if err != nil {
			log.Error("presign upload error", sl.Err(err))
			render.JSON(w, r, response.Error("presign upload error"))
			return
		}

		presignResponse := presignUploadResponse{
			key,
			url,
		}

		render.JSON(w, r, PresignUploadHTTPResponse{
			Response: response.OK(),
			PresignUploadResponse: presignResponse,
		})
	}
}