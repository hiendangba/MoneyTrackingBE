package httptransport

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"

	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/dto"
	apperrors "github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/errors"
	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
	jwtService  *service.JWTService
	logger      *slog.Logger
}

func NewAuthHandler(authService *service.AuthService, jwtService *service.JWTService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{authService: authService, jwtService: jwtService, logger: logger}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, apperrors.Validation("invalid request body"), h.logger)
		return
	}
	if err := h.authService.Register(r.Context(), req); err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusCreated, dto.MessageResponse{Message: "otp sent"})
}

func (h *AuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req dto.VerifyOTPRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, apperrors.Validation("invalid request body"), h.logger)
		return
	}
	if err := h.authService.VerifyOTP(r.Context(), req, w); err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "otp verified"})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, apperrors.Validation("invalid request body"), h.logger)
		return
	}
	if err := h.authService.Login(r.Context(), req, w); err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "logged in"})
}

func (h *AuthHandler) MobileLogin(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, apperrors.Validation("invalid request body"), h.logger)
		return
	}
	response, err := h.authService.MobileLogin(r.Context(), req)
	if err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *AuthHandler) MobileRefresh(w http.ResponseWriter, r *http.Request) {
	var req dto.MobileRefreshRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, apperrors.Validation("invalid request body"), h.logger)
		return
	}
	response, err := h.authService.MobileRefresh(r.Context(), req.RefreshToken)
	if err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *AuthHandler) MobileLogout(w http.ResponseWriter, r *http.Request) {
	var req dto.MobileLogoutRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, apperrors.Validation("invalid request body"), h.logger)
		return
	}
	accessToken := extractBearerToken(r.Header.Get("Authorization"))
	if err := h.authService.MobileLogout(r.Context(), accessToken, req.RefreshToken); err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "logged out"})
}

func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	if err := h.authService.RefreshToken(r.Context(), r, w); err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "token refreshed"})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.authService.Logout(r.Context(), r, w); err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "logged out"})
}

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req dto.ForgotPasswordRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, apperrors.Validation("invalid request body"), h.logger)
		return
	}
	if err := h.authService.ForgotPassword(r.Context(), req); err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "if the email exists, an otp has been sent"})
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req dto.ResetPasswordRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, apperrors.Validation("invalid request body"), h.logger)
		return
	}
	if err := h.authService.ResetPassword(r.Context(), req, w); err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "password reset successfully"})
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, err := UserIDFromContext(r.Context())
	if err != nil {
		writeError(w, apperrors.ErrUnauthorized, h.logger)
		return
	}
	var req dto.ChangePasswordRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, apperrors.Validation("invalid request body"), h.logger)
		return
	}
	if err := h.authService.ChangePassword(r.Context(), userID, req, w); err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "password changed successfully"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, err := UserIDFromContext(r.Context())
	if err != nil {
		writeError(w, apperrors.ErrUnauthorized, h.logger)
		return
	}
	response, err := h.authService.Me(r.Context(), userID)
	if err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "ok"})
}

func (h *AuthHandler) JWKS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=60")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.jwtService.JWKS())
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	defer func() {
		_ = r.Body.Close()
	}()

	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		return errors.New("content type must be application/json")
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func extractBearerToken(header string) string {
	header = strings.TrimSpace(header)
	const bearer = "Bearer "
	if !strings.HasPrefix(header, bearer) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, bearer))
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error, logger *slog.Logger) {
	status := apperrors.StatusCode(err)
	code := apperrors.ErrorCode(err)
	message := "internal server error"
	if status < http.StatusInternalServerError {
		message = err.Error()
	}
	if logger != nil && !errors.Is(err, apperrors.ErrUnauthorized) {
		logger.Error("request failed", "status", status, "error", err)
	}
	writeJSON(w, status, dto.ErrorResponse{
		Status:  status,
		Code:    code,
		Message: message,
	})
}
