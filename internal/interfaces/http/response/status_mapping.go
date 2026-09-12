package response

import (
	"net/http"

	"github.com/EnockYator/go-oauth/internal/shared/apperror"
)

var statusByCode = map[apperror.ErrorCode]int{
	// Generic
	apperror.CodeInternalServerError: http.StatusInternalServerError, // 500
	apperror.CodeBadRequest:          http.StatusBadRequest,          // 400
	apperror.CodeMethodNotAllowed:    http.StatusMethodNotAllowed,    // 405
	apperror.CodeUnauthorized:        http.StatusUnauthorized,        // 401
	apperror.CodeForbidden:           http.StatusForbidden,           // 403
	apperror.CodeNotFound:            http.StatusNotFound,            // 404
	apperror.CodeValidation:          http.StatusUnprocessableEntity, // 422
	apperror.CodeServiceUnavailable:  http.StatusServiceUnavailable,  // 503
	apperror.CodeConflict:            http.StatusConflict,            // 409
	apperror.CodeTooManyRequests:     http.StatusTooManyRequests,     // 429

	// Authentication / Authorization
	apperror.CodeAuthInvalidCredentials: http.StatusUnauthorized, // 401
	apperror.CodeAuthTokenExpired:       http.StatusUnauthorized, // 401
	apperror.CodeAuthTokenInvalid:       http.StatusUnauthorized, // 401
	apperror.CodeAuthTokenMissing:       http.StatusUnauthorized, // 401
	apperror.CodeAuthSessionExpired:     http.StatusUnauthorized, // 401
	apperror.CodeAuthAccountLocked:      http.StatusLocked,       // 423
	apperror.CodeAuthMFARequired:        http.StatusUnauthorized, // 401

	// User
	apperror.CodeUserNotFound:      http.StatusNotFound,            // 404
	apperror.CodeUserAlreadyExists: http.StatusConflict,            // 409
	apperror.CodeUserEmailInvalid:  http.StatusUnprocessableEntity, // 422

	// Validation
	apperror.CodeValidationRequiredField:      http.StatusUnprocessableEntity, // 422
	apperror.CodeValidationInvalidEmailFormat: http.StatusUnprocessableEntity, // 422
	apperror.CodeValidationFailed:             http.StatusUnprocessableEntity, // 422

	// Database
	apperror.CodeDBConnectionFailed: http.StatusServiceUnavailable,  // 503
	apperror.CodeDBQueryFailed:      http.StatusInternalServerError, // 500
	apperror.CodeDBRecordNotFound:   http.StatusNotFound,            // 404
	apperror.CodeDBTransactionFail:  http.StatusInternalServerError, // 500
	apperror.CodeDBConstraintFail:   http.StatusConflict,            // 409

	// External services
	apperror.CodeExternalServiceFailure: http.StatusBadGateway,         // 502
	apperror.CodeExternalTimeout:        http.StatusGatewayTimeout,     // 504
	apperror.CodeExternalUnavailable:    http.StatusServiceUnavailable, // 503

	// Payment
	apperror.CodePaymentFailed:       http.StatusPaymentRequired,     // 402
	apperror.CodePaymentDeclined:     http.StatusPaymentRequired,     // 402
	apperror.CodePaymentRequired:     http.StatusPaymentRequired,     // 402
	apperror.CodeBillingNotFound:     http.StatusNotFound,            // 404
	apperror.CodeSubscriptionExpired: http.StatusPaymentRequired,     // 402
	apperror.CodeSubscriptionInvalid: http.StatusUnprocessableEntity, // 422

	// File
	apperror.CodeFileNotFound:     http.StatusNotFound,              // 404
	apperror.CodeFileUploadFailed: http.StatusInternalServerError,   // 500
	apperror.CodeFileTooLarge:     http.StatusRequestEntityTooLarge, // 413
	apperror.CodeFileInvalidType:  http.StatusUnsupportedMediaType,  // 415

	// Storage
	apperror.CodeCloudStorageLimitReached: http.StatusInsufficientStorage, // 507
	apperror.CodeCloudStorageUnavailable:  http.StatusServiceUnavailable,  // 503

	// Abuse / security
	apperror.CodeRateLimitExceeded:    http.StatusTooManyRequests, // 429
	apperror.CodeIPBlocked:            http.StatusForbidden,       // 403
	apperror.CodeSuspiciousActivity:   http.StatusForbidden,       // 403
	apperror.CodeAuthPermissionDenied: http.StatusForbidden,       // 403

	// Infrastructure
	apperror.CodeCircuitBreakerOpen: http.StatusServiceUnavailable, // 503
	apperror.CodeDependencyFailure:  http.StatusServiceUnavailable, // 503
	apperror.CodeRequestTimeout:     http.StatusGatewayTimeout,     // 504
	apperror.CodeShutdownInProgress: http.StatusServiceUnavailable, // 503
}
