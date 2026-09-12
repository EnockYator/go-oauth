// Package apperror defines a structured error type for application-level errors,
// along with a set of stable error codes that can be used throughout the application.
//
// It provides a consistent way to represent errors, including an error code,
// message, optional details, and request/observability metadata.
package apperror

import (
	"errors"
	"fmt"
)

// ErrorCode identifies a stable application/domain-level error code.
type ErrorCode string

// AppError represents a known application-level error.
//
// AppError separates:
//   - application semantics (Code)
//   - safe client-facing information (Message)
//   - the underlying technical cause (Err)
//   - optional validation details (Details)
//   - request/observability metadata
type AppError struct {
	Code    ErrorCode     `json:"code"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`

	// Internal error (this field should never be exposed)
	Err error `json:"-"`

	// Observability metadata
	RequestID string `json:"request_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	TenantID  string `json:"tenant_id,omitempty"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e == nil {
		return "<nil>"
	}

	if e.Err == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}

	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
}

// Unwrap exposes the underlying cause to the standard errors package.
//
// This allows:
//   - errors.Is(err, target)
//   - errors.As(err, &target)
func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

// Is allows errors.Is to compare AppError codes.
//
// Example:
//
//	errors.Is(err, apperror.CodeNotFound)
//
// This provides a convenient application-level comparison.
func (e *AppError) Is(target error) bool {
	targetErr, ok := target.(*AppError)
	if !ok {
		return false
	}

	return e.Code == targetErr.Code
}

// NewCode creates a sentinel application error for use with errors.Is.
func NewCode(code ErrorCode) *AppError {
	return &AppError{
		Code: code,
	}
}

// IsCode reports whether err contains an AppError with the given code (simpler alternative to errors.Is() function).
//
// This is a convenience function for checking application error codes without
// needing to type assert the error.
//
// Example:
//
//	if apperror.IsCode(err, apperror.CodeNotFound) {
//	    // handle not found error
//	}
func IsCode(err error, code ErrorCode) bool {
	var appErr *AppError

	if !errors.As(err, &appErr) {
		return false
	}

	return appErr.Code == code
}
