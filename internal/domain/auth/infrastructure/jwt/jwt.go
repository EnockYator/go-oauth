package jwt

import "github.com/EnockYator/go-oauth/internal/domain/auth/domain"

type TokenValidator interface {
	Validate(token string) (*domain.Claims, error)
}
