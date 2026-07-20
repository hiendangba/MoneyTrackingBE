package apperrors

import (
	"errors"
	"net/http"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrEmailAlreadyExists  = errors.New("email already exists")
	ErrUserNotFound        = errors.New("user not found")
	ErrInvalidOTP          = errors.New("invalid otp")
	ErrOTPExpired          = errors.New("otp expired")
	ErrOTPAttemptsExceeded = errors.New("otp attempts exceeded")
	ErrInvalidToken        = errors.New("invalid token")
	ErrTokenBlacklisted    = errors.New("token has been revoked")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrValidation          = errors.New("validation failed")
)

type ValidationError struct {
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return e.Message
}

func StatusCode(err error) int {
	switch {
	case errors.Is(err, ErrValidation):
		return http.StatusBadRequest
	case errors.As(err, &ValidationError{}):
		return http.StatusBadRequest
	case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrInvalidOTP), errors.Is(err, ErrInvalidToken), errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrTokenBlacklisted):
		return http.StatusUnauthorized
	case errors.Is(err, ErrEmailAlreadyExists):
		return http.StatusConflict
	case errors.Is(err, ErrUserNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrOTPExpired), errors.Is(err, ErrOTPAttemptsExceeded):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
