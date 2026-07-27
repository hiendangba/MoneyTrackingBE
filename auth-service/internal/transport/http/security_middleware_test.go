package httptransport

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/config"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestExtractAccessTokenPrefersBearerOverCookie(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.Header.Set("Authorization", "Bearer bearer-token")
	request.AddCookie(&http.Cookie{Name: "access_token", Value: "cookie-token"})

	if token := extractAccessToken(request, "access_token"); token != "bearer-token" {
		t.Fatalf("extractAccessToken() = %q, want bearer-token", token)
	}
}

func TestCSRFMiddlewareRequiresOriginAndDoubleSubmitToken(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	middleware, err := NewCSRFMiddleware(config.AuthConfig{
		CSRFCookieName:     "csrf_token",
		AllowedOriginRegex: `^https://frontend\.example$`,
	}, logger)
	if err != nil {
		t.Fatalf("NewCSRFMiddleware() error = %v", err)
	}
	next := middleware.Require(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	tests := []struct {
		name   string
		origin string
		cookie string
		header string
		want   int
	}{
		{name: "missing tokens", origin: "https://frontend.example", want: http.StatusForbidden},
		{name: "wrong origin", origin: "https://evil.example", cookie: "token", header: "token", want: http.StatusForbidden},
		{name: "token mismatch", origin: "https://frontend.example", cookie: "token-a", header: "token-b", want: http.StatusForbidden},
		{name: "valid", origin: "https://frontend.example", cookie: "token", header: "token", want: http.StatusNoContent},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
			request.Header.Set("Origin", tt.origin)
			request.Header.Set("X-CSRF-Token", tt.header)
			if tt.cookie != "" {
				request.AddCookie(&http.Cookie{Name: "csrf_token", Value: tt.cookie})
			}
			response := httptest.NewRecorder()
			next.ServeHTTP(response, request)
			if response.Code != tt.want {
				t.Fatalf("status = %d, want %d", response.Code, tt.want)
			}
		})
	}
}

func TestDecodeJSONRejectsUnsafeBodies(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		contentType string
		body        string
		wantError   bool
	}{
		{name: "valid", contentType: "application/json", body: `{"value":"ok"}`},
		{name: "charset", contentType: "application/json; charset=utf-8", body: `{"value":"ok"}`},
		{name: "text plain", contentType: "text/plain", body: `{"value":"ok"}`, wantError: true},
		{name: "unknown field", contentType: "application/json", body: `{"value":"ok","unknown":true}`, wantError: true},
		{name: "multiple objects", contentType: "application/json", body: `{"value":"ok"}{"value":"second"}`, wantError: true},
		{name: "oversized", contentType: "application/json", body: `{"value":"` + strings.Repeat("a", 65<<10) + `"}`, wantError: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tt.body))
			request.Header.Set("Content-Type", tt.contentType)
			response := httptest.NewRecorder()
			var target struct {
				Value string `json:"value"`
			}
			err := decodeJSON(response, request, &target)
			if (err != nil) != tt.wantError {
				t.Fatalf("decodeJSON() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	for _, header := range []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"Referrer-Policy",
		"Permissions-Policy",
		"Content-Security-Policy",
	} {
		if response.Header().Get(header) == "" {
			t.Errorf("missing security header %s", header)
		}
	}
}

func TestRateLimiterReturnsRetryAfter(t *testing.T) {
	t.Parallel()
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewRateLimiter(redisClient, logger).Limit("test", 1, time.Minute)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/", nil))
	if first.Code != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusNoContent)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("rate-limited response is missing Retry-After")
	}
}
