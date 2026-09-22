package domain

import "github.com/google/uuid"

type UpsertUserParams struct {
    Email           string
    Name            string
    AvatarUrl       *string
    Provider        string
    ProviderSubject string
}

type GetUserByProviderSubjectParams struct {
    Provider        string
    ProviderSubject string
}

type GetUserByEmailAndProviderParams struct {
    Lower  string  // sqlc names it after the expression; verify in generated file
    Provider string
}

type UpdateUserProfileParams struct {
    ID        uuid.UUID
    Name      string
    AvatarUrl *string
}

type ListUsersParams struct {
    Limit  int32
    Offset int32
}
