package httptransport

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/config"
	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/dto"
	apperrors "github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/errors"
	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/utils"

	"github.com/redis/go-redis/v9"
)

const csrfTokenBytes = 32

var rateLimitScript = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return {count, redis.call("PTTL", KEYS[1])}
`)

type CSRFMiddleware struct {
	cfg           config.AuthConfig
	allowedOrigin *regexp.Regexp
	logger        *slog.Logger
}

func NewCSRFMiddleware(cfg config.AuthConfig, logger *slog.Logger) (*CSRFMiddleware, error) {
	allowedOrigin, err := regexp.Compile(cfg.AllowedOriginRegex)
	if err != nil {
		return nil, fmt.Errorf("compile allowed origin regex: %w", err)
	}
	return &CSRFMiddleware{cfg: cfg, allowedOrigin: allowedOrigin, logger: logger}, nil
}

func (m *CSRFMiddleware) IssueToken(w http.ResponseWriter, _ *http.Request) {
	raw := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		writeError(w, fmt.Errorf("generate csrf token: %w", err), m.logger)
		return
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	utils.SetCSRFCookie(w, m.cfg.CSRFCookieName, token, 24*time.Hour, utils.CookieConfig{
		Secure:   m.cfg.CookieSecure,
		SameSite: m.cfg.CookieSameSite,
		Domain:   m.cfg.CookieDomain,
		Path:     "/",
	})
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, dto.CSRFResponse{CSRFToken: token})
}

func (m *CSRFMiddleware) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" || !m.allowedOrigin.MatchString(origin) {
			writeError(w, apperrors.ErrForbidden, m.logger)
			return
		}

		cookieToken := utils.GetCookieValue(r, m.cfg.CSRFCookieName)
		headerToken := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
		if !constantTimeEqual(cookieToken, headerToken) {
			writeError(w, apperrors.ErrForbidden, m.logger)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func constantTimeEqual(left, right string) bool {
	if left == "" || right == "" || len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

type RateLimiter struct {
	client *redis.Client
	logger *slog.Logger
}

func NewRateLimiter(client *redis.Client, logger *slog.Logger) *RateLimiter {
	return &RateLimiter{client: client, logger: logger}
}

func (m *RateLimiter) Limit(name string, maximum int64, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity := rateLimitIdentity(r)
			key := fmt.Sprintf("auth:rate:%s:%s", name, identity)
			result, err := rateLimitScript.Run(
				r.Context(),
				m.client,
				[]string{key},
				window.Milliseconds(),
			).Slice()
			if err != nil {
				writeError(w, fmt.Errorf("enforce rate limit: %w", err), m.logger)
				return
			}
			if len(result) != 2 {
				writeError(w, errors.New("invalid redis rate limit result"), m.logger)
				return
			}
			count, countOK := result[0].(int64)
			ttlMillis, ttlOK := result[1].(int64)
			if !countOK || !ttlOK {
				writeError(w, errors.New("invalid redis rate limit result"), m.logger)
				return
			}
			if count > maximum {
				retryAfter := max(int64(1), (ttlMillis+999)/1000)
				w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
				writeError(w, apperrors.ErrTooManyRequests, m.logger)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func rateLimitIdentity(r *http.Request) string {
	ip := strings.TrimSpace(r.Header.Get("X-Envoy-External-Address"))
	if net.ParseIP(ip) == nil {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			parts := strings.Split(forwarded, ",")
			ip = strings.TrimSpace(parts[len(parts)-1])
		}
	}
	if net.ParseIP(ip) == nil {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil {
			ip = host
		} else {
			ip = r.RemoteAddr
		}
	}
	hash := sha256.Sum256([]byte(ip))
	return fmt.Sprintf("%x", hash[:16])
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
