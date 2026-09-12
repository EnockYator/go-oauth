package response

import (
	"github.com/EnockYator/go-oauth/internal/shared/apperror"
)

// APIErrorResponse defines the standard payload format for API errors.
//
// It represents the structure of an error response sent to clients.
//
// Example:
//
//	{
//	  "code": "VALIDATION_FAILED",
//	  "message": "One or more fields are invalid.",
//	  "details": [
//	    {
//	      "field": "email",
//	      "code": "INVALID_FORMAT",
//	      "message": "Must be a valid email address."
//	    },
//	    {
//	      "field": "password",
//	      "code": "TOO_SHORT",
//	      "message": "Must be at least 8 characters."
//	    }
//	  ],
//	  "request_id": "req_123"
//	}
type APIErrorResponse struct {
	Code      apperror.ErrorCode     `json:"code"`
	Message   string                 `json:"message"`
	Details   []apperror.ErrorDetail `json:"details,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
}
