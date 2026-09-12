// Package requestcontext provides type-safe helpers for storing and retrieving
// application-specific request-scoped values from context.Context.
//
// The package intentionally contains only values that are genuinely associated
// with the authenticated/request lifecycle.
//
// OpenTelemetry trace information is not stored here. Trace and span context
// are owned by the OpenTelemetry SDK and should be accessed through its APIs.
package requestcontext

import "context"

// contextKey is an unexported type used to prevent collisions with context
// keys defined by other packages.
//
// Using a distinct named type means that packages outside this package cannot
// accidentally access these values using their own context keys.
type contextKey uint8

const (
	requestIDKey contextKey = iota
	userIDKey
	tenantIDKey
	rolesKey
)

// WithRequestID returns a new context containing the request ID.
//
// The original context is not modified.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID returns the request ID stored in ctx.
//
// It returns an empty string when no request ID is present.
func GetRequestID(ctx context.Context) string {
	return getString(ctx, requestIDKey)
}

// WithUserID returns a new context containing the authenticated user ID.
//
// The original context is not modified.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// GetUserID returns the authenticated user ID stored in ctx.
//
// It returns an empty string when no user ID is present.
func GetUserID(ctx context.Context) string {
	return getString(ctx, userIDKey)
}

// WithTenantID returns a new context containing the authenticated tenant ID.
//
// The original context is not modified.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// GetTenantID returns the authenticated tenant ID stored in ctx.
//
// It returns an empty string when no tenant ID is present.
func GetTenantID(ctx context.Context) string {
	return getString(ctx, tenantIDKey)
}

// WithRoles returns a new context containing the authenticated user's roles.
//
// The original context is not modified.
//
// The roles slice is copied before being stored so that callers cannot mutate
// the request context by modifying the slice they originally supplied.
func WithRoles(ctx context.Context, roles []string) context.Context {
	if roles == nil {
		return context.WithValue(ctx, rolesKey, []string(nil))
	}

	copied := make([]string, len(roles))
	copy(copied, roles)

	return context.WithValue(ctx, rolesKey, copied)
}

// GetRoles returns a copy of the authenticated user's roles.
//
// It returns nil when no roles are present.
//
// Returning a copy prevents downstream code from mutating the slice stored
// inside the request context.
func GetRoles(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}

	roles, ok := ctx.Value(rolesKey).([]string)
	if !ok || roles == nil {
		return nil
	}

	copied := make([]string, len(roles))
	copy(copied, roles)

	return copied
}

// getString retrieves a string value from ctx.
//
// A nil context, missing value, or unexpected value type returns an empty
// string.
func getString(ctx context.Context, key contextKey) string {
	if ctx == nil {
		return ""
	}

	value, ok := ctx.Value(key).(string)
	if !ok {
		return ""
	}

	return value
}

func GetRootCtx() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	return ctx, cancel
}
