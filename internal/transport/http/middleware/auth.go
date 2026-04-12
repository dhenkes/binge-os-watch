package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

type contextKey string

const userIDKey contextKey = "userID"

// Auth returns middleware that validates the session cookie and injects
// the authenticated user ID into the request context.
func Auth(sm model.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, err := sm.Validate(r.Context(), r)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(model.AppError{
					Code:    model.ErrorCodeUnauthenticated,
					Message: "authentication required",
				})
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext extracts the authenticated user ID from the context.
func UserIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey).(string)
	return id
}

// ContextWithUserID injects a user ID into a context. Exported for tests.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}
