package errors

import (
	"errors"
	"net/http"
)

type BusinessError struct {
	Status  int
	Message string
	Code    int
}

func (e *BusinessError) Error() string { return e.Message }

func NewBusinessError(status int, message string, code int) *BusinessError {
	return &BusinessError{Status: status, Message: message, Code: code}
}

var (
	ErrValidation         = NewBusinessError(http.StatusBadRequest, "validation failed", 4000)
	ErrGroupNotFound      = NewBusinessError(http.StatusNotFound, "group not found", 4100)
	ErrGroupMemberNotFound = NewBusinessError(http.StatusNotFound, "group member not found", 4101)
	ErrInvitationNotFound = NewBusinessError(http.StatusNotFound, "invitation not found", 4102)
	ErrConflict           = NewBusinessError(http.StatusConflict, "resource already exists", 4103)
	ErrForbidden          = NewBusinessError(http.StatusForbidden, "forbidden", 4104)
	ErrGroupHasMembers    = NewBusinessError(http.StatusBadRequest, "group has members", 4105)
	ErrGroupInactive      = NewBusinessError(http.StatusBadRequest, "group is inactive", 4106)
	ErrInvitationExpired  = NewBusinessError(http.StatusBadRequest, "invitation expired", 4107)
	ErrInvitationNotPending = NewBusinessError(http.StatusBadRequest, "invitation is not pending", 4108)
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

