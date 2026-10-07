// Package repository adapts the sqlc-generated queries to the auth
// domain's ports.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	authdomain "github.com/EnockYator/go-oauth/internal/modules/auth/domain"
	"github.com/EnockYator/go-oauth/internal/modules/auth/infrastructure/database/repository/sqlc"
	"github.com/jackc/pgx/v5"
)

// SessionRepo implements authdomain.SessionRepository on top of sqlc.
type SessionRepo struct {
	q *sqlc.Queries
}

// NewSessionRepo returns a repository bound to the supplied sqlc Queries.
func NewSessionRepo(q *sqlc.Queries) *SessionRepo {
	return &SessionRepo{q: q}
}

var _ authdomain.SessionCreater = (*SessionRepo)(nil)
var _ authdomain.SessionGetter = (*SessionRepo)(nil)
var _ authdomain.SessionAdmin = (*SessionRepo)(nil)

// Create inserts a new session.
func (r *SessionRepo) CreateSession(ctx context.Context, s *authdomain.Session) error {
	if s == nil {
		return errors.New("session repo: nil session")
	}
	if s.IDHash == "" || s.UserID == "" {
		return errors.New("session repo: session missing id_hash or user_id")
	}

	metadata := s.Metadata
	if metadata == nil {
		metadata = json.RawMessage(`{}`)
	}

	row, err := r.q.CreateSession(ctx, sqlc.CreateSessionParams{
		IDHash:    s.IDHash,
		UserID:    s.UserID,
		ExpiresAt: s.ExpiresAt,
		Metadata:  metadata,
	})
	if err != nil {
		return fmt.Errorf("session repo: create: %w", err)
	}

	// The database is authoritative for created_at / last_seen.
	s.CreatedAt = row.CreatedAt
	s.LastSeen = row.LastSeen
	s.RevokedAt = row.RevokedAt
	return nil
}

// FindActiveByIDHash returns the session if it is not revoked and not
// expired. Otherwise it returns one of the domain sentinel errors.
func (r *SessionRepo) GetActiveSessionByIDHash(
	ctx context.Context,
	idHash string,
) (*authdomain.Session, error) {
	if idHash == "" {
		return nil, authdomain.ErrSessionNotFound
	}

	row, err := r.q.GetActiveSessionByIDHash(ctx, idHash)
	if err == nil {
		return mapSession(row), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("session repo: find active: %w", err)
	}

	// No *active* row. Distinguish not-found / expired / revoked so the
	// application can produce the correct error code.
	raw, rawErr := r.q.GetSessionByIDHash(ctx, idHash)
	if rawErr != nil {
		if errors.Is(rawErr, pgx.ErrNoRows) {
			return nil, authdomain.ErrSessionNotFound
		}
		return nil, fmt.Errorf("session repo: find raw: %w", rawErr)
	}

	if raw.RevokedAt != nil {
		return nil, authdomain.ErrSessionRevoked
	}
	return nil, authdomain.ErrSessionExpired
}

// RevokeByIDHash is idempotent.
func (r *SessionRepo) RevokeSessionByIDHash(ctx context.Context, idHash string) error {
	if idHash == "" {
		return nil
	}
	if _, err := r.q.RevokeSessionByIDHash(ctx, idHash); err != nil {
		return fmt.Errorf("session repo: revoke: %w", err)
	}
	return nil
}

// TouchLastSeen updates last_seen to now().
func (r *SessionRepo) TouchSessionLastSeen(ctx context.Context, idHash string) error {
	if idHash == "" {
		return nil
	}
	if _, err := r.q.TouchSessionLastSeen(ctx, idHash); err != nil {
		return fmt.Errorf("session repo: touch last seen: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------
// Mapping — no pgtype, no conversion helpers, direct field copies.
// ---------------------------------------------------------------------

func mapSession(row sqlc.Session) *authdomain.Session {
	return &authdomain.Session{
		IDHash:    row.IDHash,
		UserID:    row.UserID,
		CreatedAt: row.CreatedAt,
		ExpiresAt: row.ExpiresAt,
		RevokedAt: row.RevokedAt,
		LastSeen:  row.LastSeen,
		Metadata:  row.Metadata,
	}
}

// decodeMetadata defensively normalizes the JSONB payload to a non-nil map.
//
// The column is NOT NULL DEFAULT '{}', so an empty slice here would only
// happen in the improbable case of a hand-edited row. Returning an empty
// map rather than nil keeps callers free of nil checks.
// func decodeMetadata(raw map[string]json.RawMessage) map[string]any {
// 	if len(raw) == 0 {
// 		return map[string]any{}
// 	}
// 	var m map[string]any
// 	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
// 		return map[string]any{}
// 	}
// 	return m
// }
