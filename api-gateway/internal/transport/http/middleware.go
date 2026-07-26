package httptransport

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"

	authv1 "auth-service/gen/auth/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type contextKey string

const (
	authClaimsKey contextKey = "auth_claims"
	requestIDKey  contextKey = "request_id"
)

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if requestID == "" {
			requestID = randomToken(16)
		}
		w.Header().Set("X-Request-Id", requestID)
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func recoverMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if logger != nil {
					logger.Error("panic recovered", "panic", rec)
				}
				writeError(w, status.Error(codes.Internal, "internal server error"), logger)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(allowedOriginRegex string, next http.Handler) http.Handler {
	pattern := regexp.MustCompile(allowedOriginRegex)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" && pattern.MatchString(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token, X-Request-Id")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (g *Gateway) requireAuth(next func(http.ResponseWriter, *http.Request, AuthClaims)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			writeError(w, status.Error(codes.Unauthenticated, "missing access token"), g.logger)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), g.cfg.RequestTimeout)
		defer cancel()

		resp, err := g.authClient.ValidateAccessToken(g.upstreamContext(ctx, r, nil), &authv1.ValidateAccessTokenRequest{
			AccessToken: token,
		})
		if err != nil {
			writeError(w, err, g.logger)
			return
		}

		claims := AuthClaims{
			UserID:         resp.GetUserId(),
			RoleID:         resp.GetRoleId(),
			RoleCode:       resp.GetRoleCode(),
			SessionID:      resp.GetSessionId(),
			SessionVersion: resp.GetSessionVersion(),
		}
		next(w, r.WithContext(context.WithValue(ctx, authClaimsKey, claims)), claims)
	})
}

func (g *Gateway) requireRole(roleCode string, next func(http.ResponseWriter, *http.Request, AuthClaims)) http.Handler {
	roleCode = strings.TrimSpace(roleCode)
	return g.requireAuth(func(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
		if claims.RoleCode != roleCode {
			writeError(w, status.Error(codes.PermissionDenied, "forbidden"), g.logger)
			return
		}
		next(w, r, claims)
	})
}

func (g *Gateway) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requiresCSRF(r.Method) {
			if err := g.validateOrigin(r); err != nil {
				writeError(w, status.Error(codes.PermissionDenied, err.Error()), g.logger)
				return
			}
			cookie, err := r.Cookie(g.cfg.CSRF.CookieName)
			if err != nil || cookie.Value == "" {
				writeError(w, status.Error(codes.PermissionDenied, "missing csrf token"), g.logger)
				return
			}
			header := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
			if header == "" || !constantTimeEqual(header, cookie.Value) {
				writeError(w, status.Error(codes.PermissionDenied, "invalid csrf token"), g.logger)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (g *Gateway) validateOrigin(r *http.Request) error {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return nil
	}
	if !g.originPattern.MatchString(origin) {
		return errors.New("origin not allowed")
	}
	return nil
}

func requiresCSRF(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

func extractToken(r *http.Request) string {
	if token := extractBearerToken(r.Header.Get("Authorization")); token != "" {
		return token
	}
	if cookie, err := r.Cookie("access_token"); err == nil {
		return cookie.Value
	}
	return ""
}

func extractBearerToken(header string) string {
	header = strings.TrimSpace(header)
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}

func randomToken(size int) string {
	if size <= 0 {
		size = 32
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return base64.RawURLEncoding.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

func sameSiteFromString(raw string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "lax":
		fallthrough
	default:
		return http.SameSiteLaxMode
	}
}

func maxAgeFromTTL(ttl time.Duration) int {
	if ttl <= 0 {
		return 0
	}
	seconds := int(math.Ceil(ttl.Seconds()))
	if seconds < 0 {
		return 0
	}
	return seconds
}
