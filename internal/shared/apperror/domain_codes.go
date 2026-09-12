package apperror

// =========================
// Auth Domain
// =========================

const (
	CodeAuthInvalidCredentials ErrorCode = "AUTH_INVALID_CREDENTIALS" // 401
	CodeAuthTokenExpired       ErrorCode = "AUTH_TOKEN_EXPIRED"       // 401
	CodeAuthTokenInvalid       ErrorCode = "AUTH_TOKEN_INVALID"       // 401
	CodeAuthTokenMissing       ErrorCode = "AUTH_TOKEN_MISSING"       // 401
	CodeAuthSessionExpired     ErrorCode = "AUTH_SESSION_EXPIRED"     // 401
	CodeAuthAccountLocked      ErrorCode = "AUTH_ACCOUNT_LOCKED"      // 423
	CodeAuthMFARequired        ErrorCode = "AUTH_MFA_REQUIRED"        // 401
	CodeAuthPermissionDenied   ErrorCode = "AUTH_PERMISSION_DENIED"   // 403
	CodeIPBlocked              ErrorCode = "AUTH_IP_ADRRESS_BLOCKED"  // 403
)

// =========================
// User Domain
// =========================

const (
	CodeUserNotFound      ErrorCode = "USER_NOT_FOUND"      // 404
	CodeUserAlreadyExists ErrorCode = "USER_ALREADY_EXISTS" // 409
	CodeUserEmailInvalid  ErrorCode = "USER_EMAIL_INVALID"  // 422
)

// =========================
// Payment Domain
// =========================

const (
	CodePaymentFailed   ErrorCode = "PAYMENT_FAILED"    // 402
	CodePaymentDeclined ErrorCode = "PAYMENT_DECLINED"  // 402
	CodePaymentRequired ErrorCode = "PAYMENT_REQUIRED"  // 402
	CodeBillingNotFound ErrorCode = "BILLING_NOT_FOUND" // 404
)

// =========================
// Subscription Domain
// =========================

const (
	CodeSubscriptionExpired ErrorCode = "SUBSCRIPTION_EXPIRED" // 402
	CodeSubscriptionInvalid ErrorCode = "SUBSCRIPTION_INVALID" // 422
)

// =========================
// Storage
// =========================

const (
	CodeCloudStorageLimitReached ErrorCode = "STORAGE_LIMIT_REACHED"     // 507
	CodeCloudStorageUnavailable  ErrorCode = "CLOUD_STORAGE_UNAVAILABLE" // 503
)

// =========================
// Tenant Domain
// =========================

const (
	CodeTenantNotFound     ErrorCode = "TENANT_NOT_FOUND"     // 404
	CodeTenantInactive     ErrorCode = "TENANT_INACTIVE"      // 403
	CodeTenantIDMissing    ErrorCode = "TENANT_ID_MISSING"    // 400
	CodeTenantIDInvalid    ErrorCode = "TENANT_ID_INVALID"    // 400
	CodeTenantAccessDenied ErrorCode = "TENANT_ACCESS_DENIED" // 403
)
