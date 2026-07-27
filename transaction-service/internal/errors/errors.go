package errors

import "errors"

var (
	ErrValidation          = errors.New("validation failed")
	ErrNotFound            = errors.New("resource not found")
	ErrCategoryNotFound    = errors.New("category not found")
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrForbidden           = errors.New("forbidden")
	ErrConflict            = errors.New("resource already exists")
	ErrInvalidSplit        = errors.New("invalid transaction split")
	ErrInvalidPayment      = errors.New("invalid transaction payment")
	ErrCurrencyMismatch    = errors.New("currency mismatch")
	ErrUpstreamUnavailable = errors.New("upstream service unavailable")
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func (e *ValidationError) Unwrap() error { return ErrValidation }

func Validation(message string) error {
	return &ValidationError{Message: message}
}
