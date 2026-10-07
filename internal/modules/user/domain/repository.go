package user

import "context"

// Repository is the persistence port for users.
type UserRepository interface {
	UpsertUserByProviderSubject(ctx context.Context, params UpsertParams) (*User, error)

	GetUserByID(ctx context.Context, id string) (*User, error)
}
