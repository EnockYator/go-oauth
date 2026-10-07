package user

import "errors"

var (
	// ErrNotFound is returned when no user matches the supplied identifier.
	ErrNotFound = errors.New("user: not found")

	// ErrEmailTaken is returned when an upsert would collide with an
	// existing account on the email unique constraint.
	ErrEmailTaken = errors.New("user: email already taken")
)
