package apperrors

import (
	"errors"
	"net/http"
)

type BusinessError struct {
	Status  int
	Message string
	Code    int
}


cai

func (e *BusinessError) Error() string {
	return e.Message
}

var (
	ErrValidation          = NewBusinessError(http.StatusBadRequest, "validation failed", 4000)
	ErrInvalidCredentials  = NewBusinessError(http.StatusUnauthorized, "invalid credentials", 4001)
	ErrEmailAlreadyExists  = NewBusinessError(http.StatusConflict, "email already exists", 4002)
	ErrUserNotFound        = NewBusinessError(http.StatusNotFound, "user not found", 4003)
	ErrInvalidOTP          = NewBusinessError(http.StatusUnauthorized, "invalid otp", 4004)
	ErrOTPExpired          = NewBusinessError(http.StatusBadRequest, "otp expired", 4005)
	ErrOTPAttemptsExceeded = NewBusinessError(http.StatusBadRequest, "otp attempts exceeded", 4006)
	ErrInvalidToken        = NewBusinessError(http.StatusUnauthorized, "invalid token", 4007)
	ErrTokenBlacklisted    = NewBusinessError(http.StatusUnauthorized, "token has been revoked", 4008)
	ErrUnauthorized        = NewBusinessError(http.StatusUnauthorized, "unauthorized", 4009)
)

func Validation(message string) *BusinessError {
	return NewBusinessError(http.StatusBadRequest, message, 4000)
}

func StatusCode(err error) int {
	var businessErr *BusinessError
	if errors.As(err, &businessErr) {
		return businessErr.Status
	}
	return http.StatusInternalServerError
}

func ErrorCode(err error) int {
	var businessErr *BusinessError
	if errors.As(err, &businessErr) {
		return businessErr.Code
	}
	return 5000
}
