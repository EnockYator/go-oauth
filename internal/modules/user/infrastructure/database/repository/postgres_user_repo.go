// Package repository adapts the sqlc-generated queries to the user
// domain's ports.
package repository

import (
	"context"
	"errors"
	"fmt"

	userdomain "github.com/EnockYator/go-oauth/internal/modules/user/domain"
	"github.com/EnockYator/go-oauth/internal/modules/user/infrastructure/database/repository/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// uniqueViolation is PostgreSQL's SQLSTATE for a UNIQUE constraint failure.
const uniqueViolation = "23505"

// UserRepo implements userdomain.Repository on top of sqlc.
type UserRepo struct {
	q *sqlc.Queries
}

// NewUserRepo returns a repository bound to the supplied sqlc Queries.
func NewUserRepo(q *sqlc.Queries) *UserRepo {
	return &UserRepo{q: q}
}

var _ userdomain.UserRepository = (*UserRepo)(nil)

func (r *UserRepo) UpsertUserByProviderSubject(
	ctx context.Context,
	p userdomain.UpsertParams,
) (*userdomain.User, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	row, err := r.q.UpsertUserByProviderSubject(
		ctx,
		sqlc.UpsertUserByProviderSubjectParams{
			Email:           p.Email,
			Name:            p.Name,
			AvatarUrl:       p.AvatarURL, // *string -> nullable text
			Provider:        p.Provider,
			ProviderSubject: p.ProviderSubject,
		},
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			// (provider, provider_subject) matched no row, but the email
			// unique constraint did — another provider owns this email.
			return nil, fmt.Errorf("%w: %s", userdomain.ErrEmailTaken, p.Email)
		}
		return nil, fmt.Errorf("user repo: upsert: %w", err)
	}

	return mapUser(row), nil
}

func (r *UserRepo) GetUserByID(
	ctx context.Context,
	id string,
) (*userdomain.User, error) {
	row, err := r.q.GetUserByID(ctx, id) // string -> UUID column
	if err != nil {
		// The driver's no-rows sentinel is what distinguishes "not found"
		// from a genuine database failure. Malformed UUIDs also surface
		// here as a driver error, which we intentionally fold into
		// not-found so callers cannot probe for valid UUIDs.
		if isNoRows(err) || isInvalidInput(err) {
			return nil, userdomain.ErrNotFound
		}
		return nil, fmt.Errorf("user repo: find by id: %w", err)
	}

	return mapUser(row), nil
}

// ---------------------------------------------------------------------
// Mapping — direct field copies, no pgtype.
// ---------------------------------------------------------------------

func mapUser(row sqlc.User) *userdomain.User {
	return &userdomain.User{
		ID:              row.ID,
		Email:           row.Email,
		Name:            row.Name,
		AvatarURL:       row.AvatarUrl,
		Provider:        row.Provider,
		ProviderSubject: row.ProviderSubject,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

// ---------------------------------------------------------------------
// Driver error classification
// ---------------------------------------------------------------------

func isNoRows(err error) bool {
	return errors.Is(err, errNoRows)
}

// isInvalidInput reports whether err is PostgreSQL's SQLSTATE 22P02
// (invalid_text_representation), which is what pgx raises when a
// non-UUID string is passed to a UUID column.
func isInvalidInput(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "22P02"
	}
	return false
}

// errNoRows mirrors sql.ErrNoRows without importing database/sql, since
// the native pgx driver exposes its own sentinel via errors.Is on the
// underlying wrapped error.
var errNoRows = pgx.ErrNoRows
