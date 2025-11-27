package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/rx3lixir/laba_zis/internal/server"
)

type contextKey string

const (
	userIDKey    contextKey = "user_id"
	userEmailKey contextKey = "user_email"
	userNameKey  contextKey = "username"
)

func Middleware(authService *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				server.RespondError(w, http.StatusUnauthorized, "authorization required")
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				server.RespondError(w, http.StatusUnauthorized, "invalid authorization format")
				return
			}

			claims, err := authService.ValidateAccessToken(parts[1])
			if err != nil {
				server.RespondError(w, http.StatusUnauthorized, "invalid token")
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, userIDKey, claims.UserID)
			ctx = context.WithValue(ctx, userEmailKey, claims.Email)
			ctx = context.WithValue(ctx, userNameKey, claims.Username)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Helper functions to extract from context
func GetUserID(ctx context.Context) uuid.UUID {
	userID, _ := ctx.Value(userIDKey).(uuid.UUID)
	return userID
}

func GetEmail(ctx context.Context) string {
	email, _ := ctx.Value(userEmailKey).(string)
	return email
}

func GetUsername(ctx context.Context) string {
	username, _ := ctx.Value(userNameKey).(string)
	return username
}
