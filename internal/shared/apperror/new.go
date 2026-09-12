package apperror

import (
	"context"

	"github.com/EnockYator/go-oauth/internal/shared/requestcontext"
)

// New creates a new application error.
//
// The context is used only to capture request/observability metadata.
// HTTP status codes must not be supplied here; those are determined
// by the HTTP transport layer.
func New(
	ctx context.Context,
	code ErrorCode,
	message string,
	err error,
) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
		Details: nil,

		RequestID: requestcontext.GetRequestID(ctx),
		UserID:    requestcontext.GetUserID(ctx),
		TenantID:  requestcontext.GetTenantID(ctx),
	}
}
