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
const authRoleCodeKey contextKey = "auth_role_code"

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

		claims, err := m.jwtService.ParseAccessToken(token)
		if err != nil {
			writeError(w, apperrors.ErrUnauthorized, m.logger)
			return
		}
		ctx := context.WithValue(r.Context(), authUserIDKey, claims.Subject)
		ctx = context.WithValue(ctx, authRoleCodeKey, claims.RoleCode)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *AuthMiddleware) RequireRole(roleCode string, next http.Handler) http.Handler {
	roleCode = strings.TrimSpace(roleCode)
	return m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentRole, err := RoleCodeFromContext(r.Context())
		if err != nil || currentRole != roleCode {
			writeError(w, apperrors.ErrUnauthorized, m.logger)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func UserIDFromContext(ctx context.Context) (string, error) {
	userID, ok := ctx.Value(authUserIDKey).(string)
	if !ok || userID == "" {
		return "", errors.New("missing auth user id")
	}
	return userID, nil
}

func RoleCodeFromContext(ctx context.Context) (string, error) {
	roleCode, ok := ctx.Value(authRoleCodeKey).(string)
	if !ok || roleCode == "" {
		return "", errors.New("missing auth role code")
	}
	return roleCode, nil
}

func extractAccessToken(r *http.Request, cookieName string) string {
	if token := extractBearerToken(r.Header.Get("Authorization")); token != "" {
		return token
	}
	if cookie, err := r.Cookie(cookieName); err == nil {
		return cookie.Value
	}
	return ""
}
