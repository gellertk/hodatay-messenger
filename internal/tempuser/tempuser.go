package tempuser

import (
	"context"
	"net/http"
	"strconv"
)

type userIDKeyType struct{}

var userIDKey = userIDKeyType{}

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

// UserID: достаёт user_id из context (который положил WithUser)
func UserID(r *http.Request) int64 {
	id, _ := r.Context().Value(userIDKey).(int64)
	return id
}
