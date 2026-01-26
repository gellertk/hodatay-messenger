package uploadshandler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	response "github.com/kgellert/hodatay-messenger/internal/lib"
	"github.com/kgellert/hodatay-messenger/internal/logger/sl"
	uploadsdomain "github.com/kgellert/hodatay-messenger/internal/uploads/domain"
	userhandlers "github.com/kgellert/hodatay-messenger/internal/users/handlers"
)

type UploadsHandler struct {
	service uploadsdomain.Service
	log     *slog.Logger
}

func New(service uploadsdomain.Service, log *slog.Logger) *UploadsHandler {
	return &UploadsHandler{
		service: service,
		log:     log,
	}
}

func (h *UploadsHandler) PresignUpload() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        const op = "handlers.uploads.PresignUpload"

        log := h.log.With(
            slog.String("op", op),
            slog.String("request_id", middleware.GetReqID(r.Context())),
        )

        var req uploadsdomain.PresignUploadRequest
        if err := render.DecodeJSON(r.Body, &req); err != nil {
            log.Warn("failed to decode request", sl.Err(err))
            w.WriteHeader(http.StatusBadRequest)
            render.JSON(w, r, response.Error("invalid request body"))
            return
        }

        // Валидация ContentType
        if req.ContentType == "" {
            w.WriteHeader(http.StatusBadRequest)
            render.JSON(w, r, response.Error("content_type is required"))
            return
        }

        if !uploadsdomain.IsValidContentType(req.ContentType) {
            log.Warn("invalid content type", slog.String("content_type", req.ContentType))
            w.WriteHeader(http.StatusBadRequest)
            render.JSON(w, r, response.Error("content_type not allowed"))
            return
        }

        // Валидация Filename (опционально)
        if req.Filename != nil && len(*req.Filename) > 255 {
            w.WriteHeader(http.StatusBadRequest)
            render.JSON(w, r, response.Error("filename too long"))
            return
        }

        userID := userhandlers.UserID(r)

        fileID, url, err := h.service.PresignUpload(r.Context(), userID, req.ContentType, req.Filename)
        if err != nil {
            log.Error("failed to presign upload", sl.Err(err))
            w.WriteHeader(http.StatusInternalServerError)
            render.JSON(w, r, response.Error("failed to generate upload url"))
            return
        }

        render.JSON(w, r, uploadsdomain.PresignUploadHTTPResponse{
            Response: response.OK(),
            PresignUploadResponse: uploadsdomain.PresignUploadResponse{
                FileID:    fileID,
                UploadURL: url,
            },
        })
    }
}

func (h *UploadsHandler) ConfirmUpload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.uploads.ConfirmUpload"

		log := h.log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var req uploadsdomain.ConfirmUploadRequest
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			log.Error("invalid body", sl.Err(err))
			render.JSON(w, r, response.Error("invalid body"))
			return
		}

		userID := userhandlers.UserID(r)

		err := h.service.ConfirmUpload(r.Context(), userID, req.FileID)

		if err != nil {
			log.Error("presign upload error", sl.Err(err))
			render.JSON(w, r, response.Error("presign upload error"))
			return
		}

		render.JSON(w, r, response.OK())
	}
}
