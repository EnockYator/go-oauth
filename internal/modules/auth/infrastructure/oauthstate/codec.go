// Package oauthstate manages the transient state that must survive the
// round trip to the OAuth provider and back.
//
// The state, PKCE verifier, chosen provider, and post-login return URL are
// serialized into a single HMAC-signed cookie. Signing prevents tampering;
// confidentiality is provided by HttpOnly + Secure + the short TTL.
package oauthstate

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Payload is the data carried across the OAuth redirect.
type Payload struct {
	State        string `json:"s"`
	CodeVerifier string `json:"v"`
	Provider     string `json:"p"`
	ReturnTo     string `json:"r,omitempty"`
	IssuedAt     int64  `json:"i"`
}

// Codec signs and verifies payloads.
//
// A Codec is safe for concurrent use. It holds only immutable state.
type Codec struct {
	key []byte
	ttl time.Duration
}

// NewCodec constructs a Codec.
//
// The key MUST be at least 32 bytes; shorter keys are rejected to prevent
// deployments from accidentally using a weak secret.
func NewCodec(key []byte, ttl time.Duration) (*Codec, error) {
	if len(key) < 32 {
		return nil, errors.New("oauthstate: key must be at least 32 bytes")
	}
	if ttl <= 0 {
		return nil, errors.New("oauthstate: ttl must be positive")
	}

	// Copy the key so the caller cannot mutate it later.
	k := make([]byte, len(key))
	copy(k, key)

	return &Codec{key: k, ttl: ttl}, nil
}

// GenerateState returns a fresh, URL-safe state value.
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oauthstate: generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Encode serializes and signs the payload.
func (c *Codec) Encode(p Payload) (string, error) {
	p.IssuedAt = time.Now().Unix()

	raw, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("oauthstate: marshal payload: %w", err)
	}

	body := base64.RawURLEncoding.EncodeToString(raw)
	sig := c.sign(body)

	return body + "." + sig, nil
}

// Decode verifies and deserializes the payload.
func (c *Codec) TTL() time.Duration { return c.ttl }

func (c *Codec) Decode(s string) (*Payload, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 2 {
		return nil, errors.New("oauthstate: malformed payload")
	}

	body, sig := parts[0], parts[1]

	if !hmac.Equal([]byte(sig), []byte(c.sign(body))) {
		return nil, errors.New("oauthstate: signature mismatch")
	}

	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return nil, fmt.Errorf("oauthstate: decode body: %w", err)
	}

	var p Payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("oauthstate: unmarshal payload: %w", err)
	}

	issuedAt := time.Unix(p.IssuedAt, 0)
	now := time.Now()
	if issuedAt.After(now) {
		return nil, errors.New("oauthstate: payload issued in the future")
	}
	if now.Sub(issuedAt) > c.ttl {
		return nil, errors.New("oauthstate: payload expired")
	}

	return &p, nil
}

func (c *Codec) sign(body string) string {
	mac := hmac.New(sha256.New, c.key)
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
