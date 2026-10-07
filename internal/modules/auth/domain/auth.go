// Package auth is the module-level composition root for the auth bounded
// context. It builds every collaborator the module needs — repositories,
// provider registry, state codec, use cases — from a small set of
// primitive dependencies.
//
// The module deliberately does NOT return an HTTP handler. The interfaces
// layer (internal/interfaces/http/handler/auth) is the only place that
// knows how to adapt use cases to HTTP, and it depends on this module,
// never the reverse.
package auth

import (
	"encoding/json"
	"errors"
	"time"
)

// ProviderID is a stable identifier for an external OAuth2 identity provider.
type ProviderID string

const (
	ProviderGoogle ProviderID = "google"
	ProviderGitHub ProviderID = "github"
)

// Valid reports whether the provider is one this application supports.
func (p ProviderID) Valid() bool {
	switch p {
	case ProviderGoogle, ProviderGitHub:
		return true
	default:
		return false
	}
}

// Session is the aggregate root for an authenticated user session.
//
// The session is entirely server-side: the browser only ever holds an opaque
// random token. The database stores only SHA-256(token), so a database leak
// does not yield usable session tokens.
type Session struct {
	// IDHash is base64url(SHA-256(raw token)). Never the raw token itself.
	IDHash string

	// UserID is the users.id of the authenticated user.
	UserID string

	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
	LastSeen  time.Time

	// Metadata carries provider-specific information (e.g. the provider's
	// subject, the client's IP at session creation).
	Metadata json.RawMessage
}

type SessionWithUser struct {
	IDHash          string
	UserID          string
	CreatedAt       time.Time
	ExpiresAt       time.Time
	RevokedAt       *time.Time
	LastSeen        time.Time
	Metadata        json.RawMessage
	Email           string
	Name            string
	AvatarURL       *string
	Provider        ProviderID
	ProviderSubject string
}

// IsActive reports whether the session is usable at the given instant.
func (s *Session) IsActive(now time.Time) bool {
	if s == nil {
		return false
	}
	if s.RevokedAt != nil {
		return false
	}
	return now.Before(s.ExpiresAt)
}

// UserInfo is the normalized identity returned by any provider's userinfo
// endpoint. Providers are responsible for translating their own payload into
// this shape.
type UserInfo struct {
	Provider        ProviderID
	ProviderSubject string // stable, provider-scoped user ID (never the email)
	Email           string
	Name            string
	AvatarURL       string
}

// Validate enforces the invariants that every provider's userinfo mapping
// must satisfy. A provider that returns data failing these checks is
// considered misconfigured.
func (u *UserInfo) Validate() error {
	if u == nil {
		return errors.New("auth: nil user info")
	}
	if !u.Provider.Valid() {
		return errors.New("auth: unknown provider")
	}
	if u.ProviderSubject == "" {
		return errors.New("auth: provider subject is required")
	}
	if u.Email == "" {
		return errors.New("auth: email is required")
	}
	return nil
}
