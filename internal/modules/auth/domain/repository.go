package auth

import "context"

type SessionRepository interface {
	SessionCreater
	SessionGetter
	SessionAdmin
}

// SessionCreater is the persistence port for session creation.
//
// Implementations must be safe for concurrent use and must never store the
// raw session token — only the SHA-256 hash.
type SessionCreater interface {
	// Create inserts a new session row.
	CreateSession(ctx context.Context, s *Session) error
}

// SessionGetter is the persistence port for session finding.
type SessionGetter interface {
	GetActiveSessionByIDHash(ctx context.Context, idHash string) (*Session, error)
	// CountActiveUserSessions(ctx context.Context, userID string) (int64, error)
	// GetActiveSessionWithUserByIDHash(ctx context.Context, idHash string) (*SessionWithUser, error)
}

// SessionAdmin is the persistence port for revoking/deleting sessions.
type SessionAdmin interface {
	// ListUserActiveSessions(ctx context.Context, userID string) (*Session, error)
	RevokeSessionByIDHash(ctx context.Context, idHash string) error
	TouchSessionLastSeen(ctx context.Context, idHash string) error
	// RevokeAllUserSessions(ctx context.Context, userID string) (int64, error)
	// DeleteExpiredSessions(ctx context.Context, userID string) (int64, error)
	// RevokeUserSessionsExceptCurrent(ctx context.Context, userID string) (int64, error)
}

// type SessionRepository interface {
// 	// CreateSession inserts a new session row.
// 	CreateSession(ctx context.Context, s *Session) (*Session, error)

// 	// CountActiveUserSessions counts all active sessions associated to a specified user
// 	CountActiveUserSessions(ctx context.Context, userID string) (int64, error)

// 	// DeleteExpired sessions deletes sessions which are already revoked and expired
// 	DeleteExpiredSessions(ctx context.Context, userID string) (int64, error)

// GetActiveSessionByIDHash returns the session identified by idHash if it is
// still active (not revoked and not expired). Implementations return
// ErrSessionNotFound when no row matches, ErrSessionRevoked when the row
// exists but revoked_at is set, and ErrSessionExpired when expires_at is
// in the past.
// 	GetActiveSessionByIDHash(ctx context.Context, idHash string) (*Session, error)

// 	// GetActiveSessionWithUserByIDHash gets an active session with the user's details associated with it.
// 	// It joins user table with sessions table
// 	GetActiveSessionWithUserByIDHash(ctx context.Context, idHash string) (*SessionWithUser, error)

// 	// GetActiveSessionByIDHash gets an active session by using the id_hash
// 	GetSessionByIDHash(ctx context.Context, idHash string) (*Session, error)

// 	// ListUserActiveSessions lists all active sessions of a specified user
// 	ListUserActiveSessions(ctx context.Context, userID string) (*Session, error)

// 	// RevokeAllUserSessions revokes all session of the specified user logging him out from all devices.
// 	RevokeAllUserSessions(ctx context.Context, userID string) (int64, error)

// 	// RevokeSessionByIDHash sets revoked_at = now() on the session.
// 	// Idempotent: revoking an already-revoked or missing session is a no-op.
// 	RevokeSessionByIDHash(ctx context.Context, idHash string) (int64, error)

// 	// RevokeUserSessionsExceptCurrent revokes all sessions except the current active session
// 	RevokeUserSessionsExceptCurrent(ctx context.Context, userID string) (int64, error)

// 	// TouchSessionLastSeen updates last_seen of active session to now(). Implementations are expected
// 	// to be cheap and, where possible, non-blocking.
// 	TouchSessionLastSeen(ctx context.Context, idHash string) (int64, error)
// }
