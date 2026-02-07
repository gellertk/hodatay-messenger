package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/kgellert/hodatay-messenger/internal/logger/sl"
	"github.com/kgellert/hodatay-messenger/internal/transport/httpapi"
	"github.com/kgellert/hodatay-messenger/internal/uploads"
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
			httpapi.WriteError(w, r, err)
			return
		}

		if req.ContentType == "" {
			w.WriteHeader(http.StatusBadRequest)
			httpapi.WriteError(w, r, uploads.ErrContentTypeIsRequired)
			return
		}

		if !uploadsdomain.IsValidContentType(req.ContentType) {
			log.Warn("invalid content type", slog.String("content_type", req.ContentType))
			httpapi.WriteError(w, r, uploads.ErrInvalidContentType)
			return
		}

		// Валидация Filename (опционально)
		// if req.Filename != nil && len(*req.Filename) > 255 {
		// 	w.WriteHeader(http.StatusBadRequest)
		// 	render.JSON(w, r, response.Error("filename too long"))
		// 	return
		// }

		userID := userhandlers.UserID(r)

		pInfo, err := h.service.PresignUpload(r.Context(), userID, req.ContentType, req.Filename)
		if err != nil {
			log.Error("failed to presign upload", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		render.JSON(w, r, uploadsdomain.PresignUploadHTTPResponse{
			PresignUploadResponse: uploadsdomain.PresignUploadResponse{
				FileID:    pInfo.FileID,
				UploadURL: pInfo.URL,
				ExpiresIn: pInfo.ExpiresIn,
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
			httpapi.WriteError(w, r, err)
			return
		}

		userID := userhandlers.UserID(r)

		err := h.service.ConfirmUpload(r.Context(), userID, req.FileID)

		if err != nil {
			log.Error("presign upload error", sl.Err(err))
			httpapi.WriteError(w, r, err)
			return
		}

		render.Status(r, http.StatusNoContent)
	}
}
