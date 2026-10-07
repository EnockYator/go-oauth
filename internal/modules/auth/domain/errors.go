package auth

import "errors"

// Domain-level sentinel errors. The application layer translates these into
// apperror codes at the HTTP boundary.
var (
	// ErrSessionNotFound is returned when no active session matches the
	// supplied token hash.
	ErrSessionNotFound = errors.New("auth: session not found")

	// ErrSessionExpired is returned when a session exists but its
	// expires_at is in the past.
	ErrSessionExpired = errors.New("auth: session expired")

	// ErrSessionRevoked is returned when a session exists but has been
	// explicitly revoked.
	ErrSessionRevoked = errors.New("auth: session revoked")

	// ErrStateMismatch is returned when the OAuth2 state parameter does not
	// match the value stored in the browser's state cookie.
	ErrStateMismatch = errors.New("auth: oauth state mismatch")

	// ErrStateInvalid is returned when the state cookie is missing,
	// malformed, expired, or fails HMAC verification.
	ErrStateInvalid = errors.New("auth: oauth state invalid")

	// ErrCodeExchangeFailed is returned when the provider's token endpoint
	// rejects the authorization code exchange.
	ErrCodeExchangeFailed = errors.New("auth: oauth code exchange failed")

	// ErrUserInfoFetchFailed is returned when the provider's userinfo
	// endpoint is unreachable or returns an unexpected payload.
	ErrUserInfoFetchFailed = errors.New("auth: user info fetch failed")

	// ErrProviderUnknown is returned when a request references a provider
	// that is not registered.
	ErrProviderUnknown = errors.New("auth: unknown provider")
)
