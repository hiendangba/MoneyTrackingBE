package httptransport

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"auth-service/internal/dto"
	apperrors "auth-service/internal/errors"
	"auth-service/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
	logger      *slog.Logger
}

func NewAuthHandler(authService *service.AuthService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{authService: authService, logger: logger}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, apperrors.ValidationError{Message: "invalid request body"}, h.logger)
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
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, apperrors.ValidationError{Message: "invalid request body"}, h.logger)
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
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, apperrors.ValidationError{Message: "invalid request body"}, h.logger)
		return
	}
	if err := h.authService.Login(r.Context(), req, w); err != nil {
		writeError(w, err, h.logger)
		return
	}
	writeJSON(w, http.StatusOK, dto.MessageResponse{Message: "logged in"})
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
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, apperrors.ValidationError{Message: "invalid request body"}, h.logger)
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
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, apperrors.ValidationError{Message: "invalid request body"}, h.logger)
		return
	}
	if err := h.authService.ResetPassword(r.Context(), req, r, w); err != nil {
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
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, apperrors.ValidationError{Message: "invalid request body"}, h.logger)
		return
	}
	if err := h.authService.ChangePassword(r.Context(), userID, req, r, w); err != nil {
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

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error, logger *slog.Logger) {
	status := apperrors.StatusCode(err)
	message := "internal server error"
	if status < http.StatusInternalServerError {
		message = err.Error()
	}
	if logger != nil && !errors.Is(err, apperrors.ErrUnauthorized) {
		logger.Error("request failed", "status", status, "error", err)
	}
	writeJSON(w, status, dto.MessageResponse{Message: message})
}
