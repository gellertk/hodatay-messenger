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
	Service uploadsdomain.Service
	Log     *slog.Logger
}

func New(service uploadsdomain.Service, log *slog.Logger) *UploadsHandler {
	return &UploadsHandler{
		Service: service,
		Log:     log,
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

		userID := userhandlers.UserID(r)

		key, url, err := h.Service.PresignUpload(r.Context(), userID, req.Filename, req.ContentType)

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
			Response:              response.OK(),
			PresignUploadResponse: presignResponse,
		})
	}
}
