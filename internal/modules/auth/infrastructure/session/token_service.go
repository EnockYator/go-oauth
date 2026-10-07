package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const tokenBytes = 32

// TokenService generates and hashes opaque server-side session tokens.
// Only the hash is persisted; the raw token is returned to the HTTP layer
// exactly once so it can be placed in the browser cookie.
type TokenService struct{}

func NewTokenService() *TokenService { return &TokenService{} }

func (TokenService) Generate() (string, error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (TokenService) Hash(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
