package authHandlers

import (
	"net/http"
	"strconv"
)

func SignIn(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("user_id")
	if raw == "" {
		http.Error(w, "missing user_id", http.StatusBadRequest)
		return
	}

	uid, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || uid <= 0 {
		http.Error(w, "invalid user_id", http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "user_id",
		Value:    raw,
		Path:     "/",
		HttpOnly: true,
	})

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
