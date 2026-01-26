package uploadshandler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	response "github.com/kgellert/hodatay-messenger/internal/lib"
	"github.com/kgellert/hodatay-messenger/internal/logger/sl"
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
			render.JSON(w, r, response.Error("invalid body"))
			return
		}

		if req.FileID == "" {
			log.Error("invalid file_id")
			render.JSON(w, r, response.Error("invalid file_id"))
			return
		}

		url, err := h.service.PresignDownload(r.Context(), req.FileID)

		if err != nil {
			log.Error("presign upload error", sl.Err(err))
			render.JSON(w, r, response.Error("presign upload error"))
			return
		}

		presignResponse := uploadsdomain.PresignDownloadResponse{
			URL: url,
		}

		render.JSON(w, r, uploadsdomain.PresignDownloadHTTPResponse{
			Response:                response.OK(),
			PresignDownloadResponse: presignResponse,
		})
	}
}
