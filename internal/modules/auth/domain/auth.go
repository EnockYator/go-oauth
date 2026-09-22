package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Claims struct {
	UserID   string
	TenantID string
	Roles    []string
}

type CreateSessionParams struct {
	IDHash    string
	UserID    uuid.UUID
	ExpiresAt time.Time
	Metadata  json.RawMessage
}

type TouchSessionParams struct {
	IDHash   string
	Interval pgtype.Interval
}

type RevokeUserSessionsExceptParams struct {
	UserID uuid.UUID
	IDHash string
}
