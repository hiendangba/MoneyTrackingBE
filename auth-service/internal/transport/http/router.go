package httptransport

import (
	"log/slog"
	"net/http"

	"auth-service/internal/config"
	"auth-service/internal/service"
)

func NewRouter(cfg config.Config, logger *slog.Logger, authService *service.AuthService, jwtService *service.JWTService) http.Handler {
	handler := NewAuthHandler(authService, logger)
	authMiddleware := NewAuthMiddleware(jwtService, authService.AccessCookieName(), logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", Health)
	mux.HandleFunc("POST /api/auth/register", handler.Register)
	mux.HandleFunc("POST /api/auth/verify-otp", handler.VerifyOTP)
	mux.HandleFunc("POST /api/auth/login", handler.Login)
	mux.HandleFunc("POST /api/auth/refresh-token", handler.RefreshToken)
	mux.HandleFunc("POST /api/auth/logout", handler.Logout)
	mux.HandleFunc("POST /api/auth/forgot-password", handler.ForgotPassword)
	mux.HandleFunc("POST /api/auth/reset-password", handler.ResetPassword)
	mux.Handle("POST /api/auth/change-password", authMiddleware.RequireAuth(http.HandlerFunc(handler.ChangePassword)))
	mux.Handle("GET /api/auth/me", authMiddleware.RequireAuth(http.HandlerFunc(handler.Me)))

	return recoveryMiddleware(logger, mux)
}
