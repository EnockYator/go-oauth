// Package config loads and validates the application's runtime configuration
// from environment variables.
//
// The package is deliberately explicit: every field is populated from a
// named environment variable with a documented default, and validation is
// performed once at startup so misconfiguration never reaches production
// traffic.
package config

import "time"

// App holds the top-level application identity.
type App struct {
	AppEnv     string `validate:"required,oneof=development staging production"`
	AppName    string `validate:"required"`
	AppVersion string `validate:"required"`
}

// ServerConfig holds HTTP server tunables.
type ServerConfig struct {
	Port              int           `validate:"gte=1,lte=65535"`
	ReadTimeout       time.Duration `validate:"gt=0"`
	ReadHeaderTimeout time.Duration `validate:"gt=0"`
	WriteTimeout      time.Duration `validate:"gt=0"`
	IdleTimeout       time.Duration `validate:"gt=0"`
	ShutdownTimeout   time.Duration `validate:"gt=0"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	DBSchema string `validate:"required"`
	URL      string `validate:"required"`
	DBDriver string `validate:"required"`
}

// OAuthProviderConfig is the per-provider configuration.
//
// A provider is only "configured" when its ClientID is non-empty; a
// provider with an empty ClientID must be left nil by the loader so it is
// skipped during registry construction.
type OAuthProviderConfig struct {
	ClientID     string   `validate:"required"`
	ClientSecret string   `validate:"required"`
	RedirectURL  string   `validate:"required,url"`
	Scopes       []string `validate:"required,min=1,dive,required"`
}

// OAuthConfig is the aggregate OAuth2 + session configuration.
//
// At least one of Google or GitHub must be non-nil. This is enforced by
// ValidateConfig via a struct-level check rather than a tag, because
// go-playground/validator cannot express "exactly one or both".
type OAuthConfig struct {
	Google *OAuthProviderConfig
	GitHub *OAuthProviderConfig

	// StateSecret is the HMAC key used to sign the OAuth state cookie.
	// It must be at least 32 bytes of cryptographic randomness.
	// Generate with: openssl rand -base64 32
	StateSecret []byte `validate:"required,min=32"`

	// StateTTL bounds how long the state cookie is valid. Ten minutes is
	// generous for a human-driven consent flow.
	StateTTL time.Duration `validate:"gt=0"`

	// SessionTTL is the absolute lifetime of a user session, matching the
	// session cookie's Max-Age.
	SessionTTL time.Duration `validate:"gt=0"`

	// SessionCookieSecure controls the Secure attribute on the session
	// cookie. MUST be true outside local development, because the
	// __Host- cookie prefix requires it.
	SessionCookieSecure bool
}

// OTelConfig holds OpenTelemetry tracing configuration.
type OTelConfig struct {
	Endpoint    string
	SampleRatio float64 `validate:"gte=0,lte=1"`

	OtelShutdownTimeout time.Duration `validate:"gt=0"`
	ExportTimeout       time.Duration `validate:"gt=0"`
	Headers             map[string]string
	TLSCAFile           string
	TLSCertFile         string
	TLSKeyFile          string
	TLSServerName       string
}

// Config is the fully-loaded application configuration.
type Config struct {
	App      App
	Server   ServerConfig
	Database DatabaseConfig
	Oauth    OAuthConfig
	OTel     OTelConfig
}
