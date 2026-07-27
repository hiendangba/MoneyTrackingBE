package httptransport

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/config"
	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/service"

	"github.com/redis/go-redis/v9"
)

func NewRouter(
	cfg config.Config,
	logger *slog.Logger,
	authService *service.AuthService,
	menuService *service.MenuService,
	jwtService *service.JWTService,
	redisClient *redis.Client,
) (http.Handler, error) {
	handler := NewAuthHandler(authService, jwtService, logger)
	menuHandler := NewMenuHandler(menuService, logger)
	authMiddleware := NewAuthMiddleware(jwtService, authService.AccessCookieName(), logger)
	csrfMiddleware, err := NewCSRFMiddleware(cfg.Auth, logger)
	if err != nil {
		return nil, fmt.Errorf("create csrf middleware: %w", err)
	}
	rateLimiter := NewRateLimiter(redisClient, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", Health)
	mux.HandleFunc("GET /.well-known/jwks.json", handler.JWKS)
	mux.HandleFunc("GET /api/auth/csrf", csrfMiddleware.IssueToken)

	mux.Handle("POST /api/auth/register",
		rateLimiter.Limit("register", 3, time.Hour)(csrfMiddleware.Require(http.HandlerFunc(handler.Register))))
	mux.Handle("POST /api/auth/verify-otp",
		rateLimiter.Limit("verify-otp", 5, 10*time.Minute)(csrfMiddleware.Require(http.HandlerFunc(handler.VerifyOTP))))
	mux.Handle("POST /api/auth/login",
		rateLimiter.Limit("login", 5, 15*time.Minute)(csrfMiddleware.Require(http.HandlerFunc(handler.Login))))
	mux.Handle("POST /api/auth/refresh-token",
		rateLimiter.Limit("refresh", 30, 5*time.Minute)(csrfMiddleware.Require(http.HandlerFunc(handler.RefreshToken))))
	mux.Handle("POST /api/auth/logout",
		rateLimiter.Limit("logout", 30, 5*time.Minute)(csrfMiddleware.Require(http.HandlerFunc(handler.Logout))))
	mux.Handle("POST /api/auth/forgot-password",
		rateLimiter.Limit("forgot-password", 3, time.Hour)(csrfMiddleware.Require(http.HandlerFunc(handler.ForgotPassword))))
	mux.Handle("POST /api/auth/reset-password",
		rateLimiter.Limit("reset-password", 5, 10*time.Minute)(csrfMiddleware.Require(http.HandlerFunc(handler.ResetPassword))))

	mux.Handle("POST /api/auth/mobile/login",
		rateLimiter.Limit("mobile-login", 5, 15*time.Minute)(http.HandlerFunc(handler.MobileLogin)))
	mux.Handle("POST /api/auth/mobile/refresh",
		rateLimiter.Limit("mobile-refresh", 30, 5*time.Minute)(http.HandlerFunc(handler.MobileRefresh)))
	mux.Handle("POST /api/auth/mobile/logout",
		rateLimiter.Limit("mobile-logout", 30, 5*time.Minute)(http.HandlerFunc(handler.MobileLogout)))

	protected := rateLimiter.Limit("protected", 300, 5*time.Minute)
	mux.Handle("POST /api/auth/change-password",
		protected(csrfMiddleware.Require(authMiddleware.RequireAuth(http.HandlerFunc(handler.ChangePassword)))))
	mux.Handle("GET /api/auth/me", protected(authMiddleware.RequireAuth(http.HandlerFunc(handler.Me))))
	mux.Handle("POST /api/menus",
		protected(csrfMiddleware.Require(authMiddleware.RequireRole(cfg.Auth.AdminRoleCode, http.HandlerFunc(menuHandler.Create)))))
	mux.Handle("GET /api/menus",
		protected(authMiddleware.RequireRole(cfg.Auth.AdminRoleCode, http.HandlerFunc(menuHandler.List))))
	mux.Handle("GET /api/menus/tree",
		protected(authMiddleware.RequireAuth(http.HandlerFunc(menuHandler.Tree))))
	mux.Handle("GET /api/menus/{id}",
		protected(authMiddleware.RequireRole(cfg.Auth.AdminRoleCode, http.HandlerFunc(menuHandler.GetByID))))
	mux.Handle("PATCH /api/menus/{id}",
		protected(csrfMiddleware.Require(authMiddleware.RequireRole(cfg.Auth.AdminRoleCode, http.HandlerFunc(menuHandler.Update)))))
	mux.Handle("DELETE /api/menus/{id}",
		protected(csrfMiddleware.Require(authMiddleware.RequireRole(cfg.Auth.AdminRoleCode, http.HandlerFunc(menuHandler.Delete)))))

	return recoveryMiddleware(logger, securityHeaders(mux)), nil
}
