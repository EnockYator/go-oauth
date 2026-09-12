package apperror

// =========================
// Generic / HTTP-aligned
// =========================

const (
	CodeInternalServerError ErrorCode = "INTERNAL_SERVER_ERROR" // 500
	CodeBadRequest          ErrorCode = "BAD_REQUEST"           // 400
	CodeMethodNotAllowed    ErrorCode = "METHOD_NOT_ALLOWED"    // 405
	CodeUnauthorized        ErrorCode = "UNAUTHORIZED"          // 401
	CodeForbidden           ErrorCode = "FORBIDDEN"             // 403
	CodeNotFound            ErrorCode = "NOT_FOUND"             // 404
	CodeValidation          ErrorCode = "VALIDATION_ERROR"      // 422
	CodeServiceUnavailable  ErrorCode = "SERVICE_UNAVAILABLE"   // 503
	CodeConflict            ErrorCode = "CONFLICT"              // 409
	CodeTooManyRequests     ErrorCode = "TOO_MANY_REQUESTS"     // 429
)

// =========================
// Input Validation
// =========================

const (
	CodeValidationRequiredField      ErrorCode = "REQUIRED_FIELD"       // 422
	CodeValidationInvalidEmailFormat ErrorCode = "INVALID_EMAIL_FORMAT" // 422
	CodeValidationFailed             ErrorCode = "VALIDATION_FAILED"    // 422
)

// =========================
// Database / Persistence
// =========================

const (
	CodeDBConnectionFailed ErrorCode = "DB_CONNECTION_FAILED"    // 503
	CodeDBQueryFailed      ErrorCode = "DB_QUERY_FAILED"         // 500
	CodeDBRecordNotFound   ErrorCode = "DB_RECORD_NOT_FOUND"     // 404
	CodeDBTransactionFail  ErrorCode = "DB_TRANSACTION_FAILED"   // 500
	CodeDBConstraintFail   ErrorCode = "DB_CONSTRAINT_VIOLATION" // 409
)

// =========================
// External Services / Integrations
// =========================

const (
	CodeExternalServiceFailure ErrorCode = "EXTERNAL_SERVICE_FAILURE"     // 502
	CodeExternalTimeout        ErrorCode = "EXTERNAL_SERVICE_TIMEOUT"     // 504
	CodeExternalUnavailable    ErrorCode = "EXTERNAL_SERVICE_UNAVAILABLE" // 503
)

// =========================
// File Upload
// =========================

const (
	CodeFileNotFound     ErrorCode = "FILE_NOT_FOUND"     // 404
	CodeFileUploadFailed ErrorCode = "FILE_UPLOAD_FAILED" // 500
	CodeFileTooLarge     ErrorCode = "FILE_TOO_LARGE"     // 413
	CodeFileInvalidType  ErrorCode = "FILE_INVALID_TYPE"  // 415
)

// =========================
// Security
// =========================

const (
	CodeRateLimitExceeded  ErrorCode = "RATE_LIMIT_EXCEEDED" // 429
	CodeSuspiciousActivity ErrorCode = "SUSPICIOUS_ACTIVITY" // 403
	CodeRequestTimeout     ErrorCode = "REQUEST_TIMEOUT"     // 504 / 504
)

// =========================
// System / Infrastructure
// =========================

const (
	CodeCircuitBreakerOpen ErrorCode = "CIRCUIT_BREAKER_OPEN" // 503
	CodeDependencyFailure  ErrorCode = "DEPENDENCY_FAILURE"   // 503
	CodeShutdownInProgress ErrorCode = "SHUTDOWN_IN_PROGRESS" // 503
)
