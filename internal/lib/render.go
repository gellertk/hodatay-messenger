package response

import (
	"net/http"

	"github.com/go-chi/render"
)

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	render.Status(r, status)
	render.JSON(w, r, ErrorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: msg,
		},
	})
}