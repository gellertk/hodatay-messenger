package httpapi

import (
	"net/http"

	response "github.com/kgellert/hodatay-messenger/internal/lib"
)

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, msg := MapError(err)
	response.WriteError(w, r, status, code, msg)
}