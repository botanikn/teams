package httpapi

import (
	"context"
	"net/http"
	"strings"

	"task-api/internal/domain/errrors"
	"task-api/internal/domain/ports"
)

type contextKey string

const userIDKey contextKey = "user_id"

func authenticate(tokens ports.TokenIssuer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := strings.Fields(r.Header.Get("Authorization"))
		if len(header) != 2 || !strings.EqualFold(header[0], "Bearer") {
			writeError(w, errrors.ErrUnauthenticated)
			return
		}
		userID, err := tokens.Parse(header[1])
		if err != nil {
			writeError(w, errrors.ErrUnauthenticated)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func currentUserID(r *http.Request) int64 {
	userID, _ := r.Context().Value(userIDKey).(int64)
	return userID
}
