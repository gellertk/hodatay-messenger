package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/kgellert/hodatay-messenger/internal/logger/sl"
	"github.com/kgellert/hodatay-messenger/internal/transport/httpapi"
	errors "github.com/kgellert/hodatay-messenger/internal/uploads"
	uploadsdomain "github.com/kgellert/hodatay-messenger/internal/uploads/domain"
)

func (h *UploadsHandler) PresignDownload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.presignDownload.PresignDownload"

		log := h.log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var req uploadsdomain.PresignDownloadRequest
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			log.Error("invalid body", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		if req.FileID == "" {
			log.Error("invalid file_id")
			httpapi.WriteError(w, r, errors.ErrInvalidFileId)
			return
		}

		url, err := h.service.PresignDownload(r.Context(), req.FileID)

		if err != nil {
			log.Error("presign upload error", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		presignResponse := uploadsdomain.PresignDownloadResponse{
			URL: url,
		}

		render.JSON(w, r, uploadsdomain.PresignDownloadHTTPResponse{
			PresignDownloadResponse: presignResponse,
		})
	}
}
