package httptransport

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	apperrors "auth-service/internal/errors"
	"auth-service/internal/service"
)

type contextKey string

const authUserIDKey contextKey = "auth_user_id"

type AuthMiddleware struct {
	jwtService       *service.JWTService
	accessCookieName string
	logger           *slog.Logger
}

func NewAuthMiddleware(jwtService *service.JWTService, accessCookieName string, logger *slog.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService:       jwtService,
		accessCookieName: accessCookieName,
		logger:           logger,
	}
}

func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractAccessToken(r, m.accessCookieName)
		if token == "" {
			writeError(w, apperrors.ErrUnauthorized, m.logger)
			return
		}

		claims, err := m.jwtService.ParseAndValidate(token)
		if err != nil {
			writeError(w, apperrors.ErrUnauthorized, m.logger)
			return
		}
		if err := service.ValidateTokenType(claims, "access"); err != nil {
			writeError(w, apperrors.ErrUnauthorized, m.logger)
			return
		}

		ctx := context.WithValue(r.Context(), authUserIDKey, claims.Subject)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserIDFromContext(ctx context.Context) (string, error) {
	userID, ok := ctx.Value(authUserIDKey).(string)
	if !ok || userID == "" {
		return "", errors.New("missing auth user id")
	}
	return userID, nil
}

func extractAccessToken(r *http.Request, cookieName string) string {
	if cookie, err := r.Cookie(cookieName); err == nil {
		return cookie.Value
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "" {
		return ""
	}
	const bearer = "Bearer "
	if !strings.HasPrefix(authHeader, bearer) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(authHeader, bearer))
}
