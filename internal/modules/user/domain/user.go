// Package user contains the domain model for the application's users,
// independent of any identity provider.
package user

import (
	"errors"
	"strings"
	"time"
)

// User is a resource owner known to the application.
type User struct {
	ID              string
	Email           string
	Name            string
	AvatarURL       *string // NULL in DB <-> nil in Go
	Provider        string
	ProviderSubject string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// UpsertParams are the fields needed to create-or-refresh a user from an
// OAuth provider's userinfo response.
type UpsertParams struct {
	Email           string
	Name            string
	AvatarURL       *string
	Provider        string
	ProviderSubject string
}

// Validate enforces the invariants the database also protects.
func (p UpsertParams) Validate() error {
	if strings.TrimSpace(p.Email) == "" {
		return errors.New("user: email is required")
	}
	if strings.TrimSpace(p.Provider) == "" {
		return errors.New("user: provider is required")
	}
	if strings.TrimSpace(p.ProviderSubject) == "" {
		return errors.New("user: provider subject is required")
	}
	return nil
}
