package userhandlers

import (
	"context"
	"net/http"
	"strconv"
)

type userIDKeyType struct{}

var userIDKey = userIDKeyType{}

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

// WithUser: берёт user_id из COOKIE "user_id"
func WithUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("user_id")
		if err != nil || c.Value == "" {
			http.Error(w, "missing user_id", http.StatusBadRequest)
			return
		}

		uid, err := strconv.ParseInt(c.Value, 10, 64)
		if err != nil || uid <= 0 {
			http.Error(w, "invalid user_id", http.StatusBadRequest)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserID(r *http.Request) int64 {
	id, _ := r.Context().Value(userIDKey).(int64)
	return id
}
