# go-oauth

`go-oauth` is a Go authentication service that uses   **`OAuth2.0`** via `golang.org/x/oauth2` to authenticate users through **GitHub** and **Google** auth providers, then issue **opaque server-side sessions** for the rest of the user's visit.

**NOTE:** No passwords required, no JWT, no manual access-token / refresh-token management.

---

## Table of Contents

- [go-oauth](#go-oauth)
	- [Table of Contents](#table-of-contents)
	- [Overview](#overview)
	- [Identity Providers](#identity-providers)
	- [Features](#features)
	- [Prerequisites](#prerequisites)
	- [Getting Started](#getting-started)
		- [1. Clone the repository \& install the required dependencies](#1-clone-the-repository--install-the-required-dependencies)
		- [2. Register OAuth applications](#2-register-oauth-applications)
			- [Google](#google)
			- [GitHub](#github)
		- [3. Create the PostgreSQL database](#3-create-the-postgresql-database)
		- [4. Configure environment variables](#4-configure-environment-variables)
		- [5. Run the application](#5-run-the-application)
	- [Component Responsibilities](#component-responsibilities)
	- [Environment Configuration Reference](#environment-configuration-reference)
	- [Architecture](#architecture)
	- [API Endpoints](#api-endpoints)
		- [Example: `/api/me` response](#example-apime-response)
		- [Example: initiating login](#example-initiating-login)
	- [Project Structure](#project-structure)
	- [Testing](#testing)
	- [Security Considerations](#security-considerations)
	- [Troubleshooting](#troubleshooting)
	- [Contributing](#contributing)
		- [Adding a new provider](#adding-a-new-provider)
	- [License](#license)
	- [Acknowledgements](#acknowledgements)

---

## Overview

`go-oauth` implements the **OAuth 2.0 authorization code flow** using the official Go OAuth 2.0 client library. Users sign in through an external Identity Provider (GitHub or Google). The provider returns an authorization code, the server exchanges it for a token, fetches the user's profile, and then **immediately discards** the **OAuth tokens**.

From that point on, the user's session is represented by an **opaque session ID** stored in a server-side session store (in this case PostgreSQL `sessions table`) and delivered to the browser as an HTTP-only cookie. There is no JWT parsing, no refresh-token rotation, and no client-side token handling.

---

## Identity Providers

| Provider | Authorization Endpoint |	User Info Source |
|----------|------------------------|------------------|
| **Google** | `accounts.google.com/o/oauth2/auth` |  `oauth2/google → Userinfo` |
| **GitHub**	| `github.com/login/oauth/authorize` | `api.github.com/user` |

The providers are configured in `./internal/modules/auth/infrastructure/oauth/{provider}/{provider}/.go` files through `oauth2.Config` from `golang.org/x/oauth2`.

---

## Features

- **Google & GitHub providers** — registered via `oauth2.Config`
- **Extensibility** - uses a pluggable provider interface for adding more authentication providers.
- **Standard library approach** — uses `golang.org/x/oauth2` directly; no third-party auth frameworks.
- **CSRF protection** — cryptographically random `state` parameter with server-side session validation.
- **PKCE support** — enabled by default for both providers (`S256` code challenge).
- **Server-side session store** — stores sessions in PostgreSQL.
- **Secure session management** — signed, HTTP-only, `SameSite=Lax` cookies with configurable TTL.
- **Account linking** — link multiple providers to a single user identity by verified email.
- **Logout** — simply deletes the session record and clears the cookie
- **Structured logging** — `slog` with trace correlation across the OAuth flow.
- **OpenTelemetry tracing** — spans for each leg of the authorization code exchange.
- **Graceful shutdown** — drains in-flight requests and flushes spans on `SIGTERM`.

---

## Prerequisites

- **Go 1.22+** — uses `http.ServeMux` method patterns and `log/slog`.
- **A Google Cloud project** — for Google OAuth credentials.
- **A GitHub OAuth App** — for GitHub credentials.
- **PostgreSQL** — for database persistence

---

## Getting Started

### 1. Clone the repository & install the required dependencies

```bash
git clone https://github.com/EnockYator/go-oauth.git

cd go-oauth

go mod download

go mod tidy
```

### 2. Register OAuth applications

#### Google

1. Go to the [Google Cloud Console](https://console.cloud.google.com/apis/credentials).
2. Create a new project (or select an existing one).
3. Navigate to **APIs & Services → Credentials → Create Credentials → OAuth client ID**.
4. Choose **Web application**.
5. Add an **Authorized redirect URI**:
   ```
   http://localhost:8080/auth/google/callback
   ```
6. Copy the **Client ID** and **Client Secret**.

#### GitHub

1. Go to [GitHub Developer Settings → OAuth Apps](https://github.com/settings/developers).
2. Click **New OAuth App**.
3. Set the **Authorization callback URL**:
   ```
   http://localhost:8080/auth/github/callback
   ```
4. Copy the **Client ID** and generate a **Client Secret**.

### 3. Create the PostgreSQL database

Follow psql best practices in creating and handling the database.

### 4. Configure environment variables

Copy the .env.example file below and fill in your credentials replacing all the placeholders such as, `<YOUR_DB_USER>`:

```bash
cp .env.example .env.development
```

```dotenv
# ===============================
# App
# ===============================
APP_ENV=development     # APP_ENV=production in production
APP_NAME=go_oauth
APP_VERSION=1.0.0
# ===============================
# Server
# ===============================
SERVER_PORT=8080
SERVER_READ_TIMEOUT=10s
SERVER_READ_HEADER_TIMEOUT=5s
SERVER_WRITE_TIMEOUT=30s
SERVER_IDLE_TIMEOUT=120s
SERVER_SHUTDOWN_TIMEOUT=30s

# ===============================
# PostgreSQL DB
# ===============================
POSTGRES_DB_SCHEMA=public
POSTGRES_URL=postgres://<YOUR_DB_USER>:<YOUR_DB_PASSWORD>@<DB_HOST>:<DB_PORT>/<DB_NAME>?sslmode=disable   # sslmode=require in production
POSTGRES_DRIVER=pgx

# ===============================
# CI Test Database
# ===============================
TEST_DB_USER=<TEST_DB_USER>
TEST_DB_PASSWORD=<TEST_DB_PASSWORD>
TEST_DB_NAME=<TEST_DB_NAME>
TEST_DB_URL=postgres://<TEST_DB_USER>:<TEST_DB_PASSWORD>@<TEST_DB_HOST>:<TEST_DB_PORT>/<TEST_DB_NAME>?sslmode=disable

# ===============================
# Oauth
# ===============================
OAUTH_GOOGLE_CLIENT_ID=<YOUR_GOOGLE_CLIENT_ID>
OAUTH_GOOGLE_CLIENT_SECRET=<YOUR_GOOGLE_CLIENT_SECRET>
OAUTH_GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback
OAUTH_GOOGLE_SCOPES=openid,email,profile

OAUTH_GITHUB_CLIENT_ID=<YOUR_GITHUB_CLIENT_ID>
OAUTH_GITHUB_CLIENT_SECRET=<YOUR_GITHUB_CLIENT_SECRET>
OAUTH_GITHUB_REDIRECT_URL=http://localhost:8080/auth/github/callback
OAUTH_GITHUB_SCOPES=read:user,user:email

OAUTH_STATE_SECRET=<YOUR_OAUTH_STATE_SECRET>     # base64 of 32+ random bytes
OAUTH_STATE_TTL=10m
OAUTH_SESSION_TTL=720h
OAUTH_SESSION_SECURE=false          # true in production, false in development

# ===============================
# OTel
# ===============================
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
OTEL_TRACES_SAMPLER_RATIO=0.1
OTEL_SHUTDOWN_TIMEOUT=5s
OTEL_EXPORT_TIMEOUT=10s

OTEL_EXPORTER_OTLP_HEADERS=
OTEL_EXPORTER_OTLP_CA_FILE=
OTEL_EXPORTER_OTLP_CERT_FILE=
OTEL_EXPORTER_OTLP_KEY_FILE=
OTEL_EXPORTER_OTLP_SERVER_NAME=
```

### 5. Run the application

```bash
# Development
go run ./cmd/api/

# Or build and run the binary
go build -o bin/api ./cmd/api

./bin/api
```

The server starts on `http://localhost:8080`. Open your browser and navigate to:

- `http://localhost:8080/auth/google/login` — to sign in with Google
- `http://localhost:8080/auth/github/login` — to sign in with GitHub
- `http://localhost:8080/api/me` — to view the authenticated user (requires a session)

---

## Component Responsibilities

| Component | Owns | Does not own |
|-----------|------|--------------|
| Browser | The session cookie(opaque token) | Any session data, or user info |
| go-oauth API | Business logic, OAuth flow, session issuance, token generation | Persistence details (delegated to `*store` interfaces) |
| PostgreSQL | User records, session records, provider identity mappings | Validations, authorization decisions |
| Google / GitHub (Providers) | Identity verification, access tokens for their APIS | session / user record, authorization |

---

## Environment Configuration Reference

| Variable | Description |
|---|---|
| `APP_ENV` | `development` or `production`. Controls log format and cookie `Secure` flag. |
| `APP_NAME` | Service name used in logs, traces, and metrics. Also emitted as the OTel `service.name` resource attribute. |
| `APP_VERSION` | Build version. Emitted as the OTel `service.version` resource attribute for deploy correlation. |
| `SERVER_PORT` | TCP port the HTTP server listens on. |
| `SERVER_READ_TIMEOUT` | Maximum duration for reading the entire request, including the body. Accepts Go duration strings. |
| `SERVER_READ_HEADER_TIMEOUT` | Maximum duration for reading request headers. Guards against Slowloris-style attacks. |
| `SERVER_WRITE_TIMEOUT` | Maximum duration before timing out writes of the response. |
| `SERVER_IDLE_TIMEOUT` | Maximum duration to wait for the next request when keep-alives are enabled. |
| `SERVER_SHUTDOWN_TIMEOUT` | Maximum duration to wait for in-flight requests to drain during graceful shutdown before forcing exit. |
| `POSTGRES_DB_SCHEMA` | Default schema used for unqualified table names. |
| `POSTGRES_URL` | PostgreSQL connection string. Format: `postgres://<user>:<password>@<host>:<port>/<db>?sslmode=<mode>`. Use `sslmode=require` or `verify-full` in production. |
| `POSTGRES_DRIVER` | Driver registered with `database/sql` via `pgx/v5/stdlib`. Do not change unless swapping the driver implementation. |
| `TEST_DB_USER` | Username for the ephemeral CI test database. Only read when running integration tests. |
| `TEST_DB_PASSWORD` | Password for the CI test database. Only read when running integration tests. |
| `TEST_DB_NAME` | Database name for the CI test database. Only read when running integration tests. |
| `TEST_DB_URL` | Full DSN for the CI test database. Format: `postgres://<user>:<password>@<host>:<port>/<db>?sslmode=disable`. Never point this at a production database. |
| `GOOGLE_CLIENT_ID` | Google OAuth 2.0 client ID from the Google Cloud Console. |
| `GOOGLE_CLIENT_SECRET` | Google OAuth 2.0 client secret. Keep out of source control. |
| `GOOGLE_REDIRECT_URL` | Callback URL registered in Google Cloud Console. Must match exactly (scheme, host, port, path). |
| `GOOGLE_SCOPES` | Comma-separated OAuth scopes requested during login. |
| `GITHUB_CLIENT_ID` | GitHub OAuth App client ID. |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth App client secret. Keep out of source control. |
| `GITHUB_REDIRECT_URL` | Callback URL registered in the GitHub OAuth App. Must match exactly. |
| `GITHUB_SCOPES` | Comma-separated OAuth scopes. `user:email` is required to read the primary verified email. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP gRPC endpoint for trace export. Empty value disables tracing entirely. |
| `OTEL_TRACES_SAMPLER_RATIO` | Head sampling ratio between `0.0` and `1.0`. `0.1` samples 10% of traces. Use `1.0` in development. |
| `OTEL_SHUTDOWN_TIMEOUT` | Maximum duration to flush pending spans to the collector during shutdown. |
| `OTEL_EXPORTER_OTLP_HEADERS` | Comma-separated `key=value` headers sent with OTLP exports. Use for auth tokens when exporting to a hosted backend (Honeycomb, Grafana Cloud, etc.). |
| `OTEL_EXPORTER_OTLP_CA_FILE` | Path to a PEM-encoded CA certificate for verifying the OTLP collector's TLS certificate. Required when the collector uses a private CA. |
| `OTEL_EXPORTER_OTLP_CERT_FILE` | Path to a PEM-encoded client certificate for mutual TLS to the OTLP collector. Required only when the collector enforces mTLS. |
| `OTEL_EXPORTER_OTLP_KEY_FILE` | Path to the PEM-encoded private key matching `OTEL_EXPORTER_OTLP_CERT_FILE`. Required only when the collector enforces mTLS. |
| `OTEL_EXPORTER_OTLP_SERVER_NAME` | Expected server name (SNI) when verifying the OTLP collector's certificate. Set this when the endpoint's hostname differs from the certificate's CN/SAN. |

---

## Architecture

Read more on [Architecture](ARCHITECTURE.md)

---

## API Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/auth/{provider}/login` | Initiates the OAuth flow. Redirects to the provider. |
| `GET` | `/auth/{provider}/callback` | Handles the provider's redirect, exchanges the code, creates a session. |
| `POST` | `/auth/logout` | Clears the session cookie and invalidates the server-side session. |
| `GET` | `/api/me` | Returns the authenticated user as JSON. `401` if no session. |
| `GET` | `/healthz` | Liveness probe. Returns `200 OK` while the process is running. |
| `GET` | `/readyz` | Readiness probe. Checks PostgreSQL and provider reachability. |

Supported `{provider}` values: `google`, `github`.

### Example: `/api/me` response

```json
{
  "id": "usr_01HX8Z9K2M4N6P8R0T2V4W6Y8A",
  "email": "amos@example.com",
  "name": "Amos James",
  "avatar_url": "https://lh3.googleusercontent.com/a/...",
  "providers": [
    { "name": "google", "subject": "1029384756...", "linked_at": "2024-06-25T03:02:16Z" }
  ],
  "created_at": "2024-06-25T03:02:16Z"
}
```

### Example: initiating login

```bash
# Redirects the browser to Google's consent screen.
curl -i http://localhost:8080/auth/google/login
```

---

## Project Structure

```
go-oauth
├── ARCHITECTURE.md
├── cmd
│   └── api
│       └── main.go
├── docs
│   ├── docs.go
│   ├── swagger.json
│   └── swagger.yaml
├── go.mod
├── go.sum
├── internal
│   ├── bootstrap
│   │   ├── application.go
│   │   ├── auth.go
│   │   ├── database.go
│   │   ├── http.go
│   │   └── observability.go
│   ├── infrastructure
│   │   ├── config
│   │   │   ├── config.go
│   │   │   ├── getters.go
│   │   │   ├── loader.go
│   │   │   └── validator.go
│   │   ├── database
│   │   │   └── postgres
│   │   │       ├── connect.go
│   │   │       └── migrations
│   │   │           ├── 000001_create_extensions.down.sql
│   │   │           ├── 000001_create_extensions.up.sql
│   │   │           ├── 000002_create_functions.down.sql
│   │   │           ├── 000002_create_functions.up.sql
│   │   │           ├── 000003_create_users_table.down.sql
│   │   │           ├── 000003_create_users_table.up.sql
│   │   │           ├── 000004_create_sessions_table.down.sql
│   │   │           └── 000004_create_sessions_table.up.sql
│   │   └── observability
│   │       ├── oteltracing
│   │       │   ├── config.go
│   │       │   ├── logger.go
│   │       │   └── tracing.go
│   │       └── prometheus
│   ├── interfaces
│   │   └── http
│   │       ├── cookie
│   │       │   └── session.go
│   │       ├── dto
│   │       │   ├── auth
│   │       │   │   ├── auth_request.go
│   │       │   │   └── auth_response.go
│   │       │   ├── health
│   │       │   │   └── health.go
│   │       │   └── root
│   │       │       └── root.go
│   │       ├── handler
│   │       │   ├── auth
│   │       │   │   └── auth_handler.go
│   │       │   ├── health
│   │       │   │   └── health.go
│   │       │   └── root
│   │       │       └── root.go
│   │       ├── middleware
│   │       │   ├── auth.go
│   │       │   ├── auth_test.go
│   │       │   ├── cors.go
│   │       │   ├── cors_test.go
│   │       │   ├── logger.go
│   │       │   ├── rate_limit.go
│   │       │   ├── rate_limit_test.go
│   │       │   ├── recovery.go
│   │       │   ├── recovery_test.go
│   │       │   ├── request_id.go
│   │       │   ├── request_id_test.go
│   │       │   ├── response_recorder.go
│   │       │   ├── tenant.go
│   │       │   ├── tenant_test.go
│   │       │   ├── timeout.go
│   │       │   ├── timeout_test.go
│   │       │   ├── trace.go
│   │       │   └── trace_test.go
│   │       ├── response
│   │       │   ├── api_error_response.go
│   │       │   ├── api_success_response.go
│   │       │   ├── error.go
│   │       │   ├── response.go
│   │       │   └── status_mapping.go
│   │       ├── router.go
│   │       └── server.go
│   ├── modules
│   │   ├── auth
│   │   │   ├── application
│   │   │   │   ├── handle_callback.go
│   │   │   │   ├── initiate_login.go
│   │   │   │   ├── logout.go
│   │   │   │   ├── ports.go
│   │   │   │   └── validate_session.go
│   │   │   ├── domain
│   │   │   │   ├── auth.go
│   │   │   │   ├── errors.go
│   │   │   │   └── repository.go
│   │   │   └── infrastructure
│   │   │       ├── database
│   │   │       │   ├── queries
│   │   │       │   │   └── sessions.sql
│   │   │       │   └── repository
│   │   │       │       ├── postgres_session_repo.go
│   │   │       │       └── sqlc
│   │   │       │           ├── db.go
│   │   │       │           ├── models.go
│   │   │       │           └── sessions.sql.go
│   │   │       ├── oauth
│   │   │       │   ├── application_adapter.go
│   │   │       │   ├── factory.go
│   │   │       │   ├── github
│   │   │       │   │   └── github.go
│   │   │       │   ├── google
│   │   │       │   │   └── google.go
│   │   │       │   ├── provider.go
│   │   │       │   └── registry.go
│   │   │       ├── oauthstate
│   │   │       │   ├── application_adapter.go
│   │   │       │   └── codec.go
│   │   │       └── session
│   │   │           └── token_service.go
│   │   └── user
│   │       ├── application
│   │       │   └── upsert_from_oauth.go
│   │       ├── domain
│   │       │   ├── errors.go
│   │       │   ├── repository.go
│   │       │   └── user.go
│   │       └── infrastructure
│   │           └── database
│   │               ├── queries
│   │               │   └── users.sql
│   │               └── repository
│   │                   ├── postgres_user_repo.go
│   │                   └── sqlc
│   │                       ├── db.go
│   │                       ├── models.go
│   │                       └── users.sql.go
│   └── shared
│       ├── apperror
│       │   ├── apperror.go
│       │   ├── application_codes.go
│       │   ├── domain_codes.go
│       │   ├── errordetail.go
│       │   └── new.go
│       └── requestcontext
│           └── request_context.go
├── LICENSE
├── Makefile
├── README.md
└── sqlc.yaml
```

The structure follows strict separation of concerns: **domains/modules** (`internal/modules/user`, `internal/modules/auth`) knows nothing about HTTP or OAuth libraries; **infrastructure** (`internal/infrastructure`) knows nothing about business rules; **interface** (`internal/interfaces/http`) translates between HTTP and the domain.

---

## Testing

```bash
# Run all tests
go test ./...

# Run tests with race detector
go test -race ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```
---

## Security Considerations

This service handles identity. Treat every line of code as security-critical.

**State parameter (CSRF protection).**
A 32-byte cryptographically random string is generated per login attempt, stored in the session, and validated on callback. A mismatch is rejected with `400 Bad Request` and logged at `Warn`. Never disable this check.

**PKCE (Proof Key for Code Exchange).**
Enabled by default for both providers, even though the flow is server-side. PKCE protects against authorization-code interception in environments where the redirect URI is not fully trusted (mobile, SPAs, multi-tenant proxies).

**Session cookies.**
Signed with HMAC-SHA256 using `SESSION_SECRET`. In production (`APP_ENV=production`), cookies are set with `Secure`, `HttpOnly`, and `SameSite=Lax`. Rotate `SESSION_SECRET` during incident response — this invalidates all sessions immediately.

**Secret handling.**
Client secrets are read from the environment and never logged. The `config.String()` method redacts them. Do not commit `.env` files; `.gitignore` blocks them by default.

**Redirect URI validation.**
Every provider's `RedirectURL` must **exactly** match the value registered with the provider (scheme, host, port, and path). A mismatch causes a `redirect_uri_mismatch` error during the token exchange.

**Email verification.**
Both Google and GitHub may return unverified emails. The service only links identities when the provider reports `email_verified: true` (Google) or the email is the primary verified address (GitHub). Unverified emails are rejected to prevent account takeover through email spoofing.

**Rate limiting.**
Apply rate limits at your reverse proxy or ingress for `/auth/*/login` and `/auth/*/callback`. The service includes a basic built-in rate-limiter which may not be reliable; use Cloudflare, nginx, or an API gateway.

**TLS.**
Terminate TLS at your load balancer. In production, the service itself should also serve HTTPS or run behind a trusted proxy that sets `X-Forwarded-Proto`.

---

## Troubleshooting

**`redirect_uri_mismatch` from Google**
The `GOOGLE_REDIRECT_URL` does not match a URI registered in the Google Cloud Console. Add the exact URI, including scheme and port, and wait up to 5 minutes for propagation.

**`bad_verification_code` from GitHub**
The authorization code has already been used or expired. Codes are single-use; do not refresh the callback URL. Restart the flow from `/auth/github/login`.

**`invalid state parameter`**
The session cookie was lost between the login redirect and the callback. Common causes: cookie `Secure` flag set while running over plain HTTP, `SameSite=Strict` blocking the cross-site redirect, or a different `SESSION_SECRET` across replicas. Ensure `APP_ENV=development` for local HTTP and that all instances share the same secret.

**Empty email from GitHub**
GitHub only returns the primary email when the `user:email` scope is granted. Verify `GITHUB_SCOPES` includes `user:email` and that the user has a verified primary email on their account.

**Traces not appearing**
Confirm `OTEL_EXPORTER_OTLP_ENDPOINT` is reachable from the container. Check for `opentelemetry sdk error` log lines — the SDK error handler routes export failures into `slog` at `Error` level.

**Sessions lost on restart**
You are using cookie sessions (no `REDIS_URL`). This is expected — cookie sessions are stateless and survive restarts, but rotating `SESSION_SECRET` invalidates them. If you need cross-restart persistence, configure Redis.

---

## Contributing

1. Fork the repository and create a feature branch.
2. Follow the existing layering/structure: domain → application → infrastructure → interface.
3. Every new provider must implement the `auth.Provider` interface and register itself in `/internal/modules/auth/infrastructure/oauth/registry.go`.
4. Add unit tests using the `httptest`-based stubs. No live network calls in `go test`.
5. Run `go vet ./...` and `golangci-lint run` before opening a pull request.
6. Update this README if you add configuration variables or endpoints.

### Adding a new provider

Create the file: `/internal/modules/auth/infrastructure/oauth/{your_provider}/{your_provider}.go`

```go
// Package <your_provider> implements the oauth.Provider port for <your_provider>'s OIDC
// endpoints.
package <your_provider>

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/EnockYator/go-oauth/internal/modules/auth/domain"
	"golang.org/x/oauth2"
)

const (
	authURL     = "your_provider's_auth_url"
	tokenURL    = "your_provider's_token_url"
	userInfoURL = "your_provider's_user_info_url"
)

// Config is the per-provider configuration for Google.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// Provider implements the oauth.Provider interface for the specific provider.
type Provider struct {
	cfg        *oauth2.Config
	httpClient *http.Client
}

// New constructs a <your_provider> provider. The httpClient is used for both the
// token exchange and the userinfo request, ensuring a shared timeout and
// connection pool.
func New(cfg Config, httpClient *http.Client) *Provider {
	return &Provider{
		cfg: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  authURL,
				TokenURL: tokenURL,
			},
		},
		httpClient: httpClient,
	}
}

func (p *Provider) ID() auth.ProviderID { return auth.Provider<Your_provider> }

func (p *Provider) AuthCodeURL(
	state string,
	opts ...oauth2.AuthCodeOption,
) string {
	return p.cfg.AuthCodeURL(state, opts...)
}

func (p *Provider) Exchange(
	ctx context.Context,
	code string,
	opts ...oauth2.AuthCodeOption,
) (*oauth2.Token, error) {
	// oauth2 reads the HTTP client from this context key.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)
	return p.cfg.Exchange(ctx, code, opts...)
}

// userInfoResponse is the provider's OIDC /userinfo payload (relevant fields only).
type userInfoResponse struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (p *Provider) FetchUserInfo(
	ctx context.Context,
	token *oauth2.Token,
) (*auth.UserInfo, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)
	client := p.cfg.Client(ctx, token)

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, userInfoURL, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("google: build userinfo request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google: call userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"yourProvider: userinfo returned status %d", resp.StatusCode,
		)
	}

	var body userInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("yourProvider: decode userinfo: %w", err)
	}

	if body.Sub == "" {
		return nil, fmt.Errorf("yourProvider: userinfo missing 'sub'")
	}
	if body.Email == "" {
		return nil, fmt.Errorf("yourProvider: userinfo missing 'email'")
	}
	// Reject unverified emails: an attacker who controls an unverified
	// yourProvider address could otherwise impersonate a user.
	if !body.EmailVerified {
		return nil, fmt.Errorf("yourProvider: email is not verified")
	}

	return &auth.UserInfo{
		Provider:        auth.Provider<Your_provider>,
		ProviderSubject: body.Sub,
		Email:           body.Email,
		Name:            body.Name,
		AvatarURL:       body.Picture,
	}, nil
}
```

Then update `/internal/modules/auth/infrastructure/oauth/factory.go`:

```go
package oauth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/EnockYator/go-oauth/internal/modules/auth/domain"
	"github.com/EnockYator/go-oauth/internal/modules/auth/infrastructure/oauth/github"
	"github.com/EnockYator/go-oauth/internal/modules/auth/infrastructure/oauth/google"
	"github.com/EnockYator/go-oauth/internal/modules/auth/infrastructure/oauth/<your_provider>"
)

// BuildConfig is the top-level configuration for the provider registry.
type BuildConfig struct {
	// HTTPClient is used for all outbound provider calls. If nil, a client
	// with a 10-second timeout is created. ALWAYS provide a client with a
	// timeout — oauth2's default is http.DefaultClient, which has none.
	HTTPClient *http.Client

	Google *ProviderConfig
	GitHub *ProviderConfig
	<YourProvider> *ProviderConfig
}

// Build constructs a Registry from the supplied configuration.
func Build(cfg BuildConfig) (*Registry, error) {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	reg := NewRegistry()

	if cfg.Google != nil {
		if err := validateConfig(auth.ProviderGoogle, cfg.Google); err != nil {
			return nil, err
		}
		reg.Register(google.New(google.Config{
			ClientID:     cfg.Google.ClientID,
			ClientSecret: cfg.Google.ClientSecret,
			RedirectURL:  cfg.Google.RedirectURL,
			Scopes:       cfg.Google.Scopes,
		}, httpClient))
	}

	if cfg.GitHub != nil {
		if err := validateConfig(auth.ProviderGitHub, cfg.GitHub); err != nil {
			return nil, err
		}
		reg.Register(github.New(github.Config{
			ClientID:     cfg.GitHub.ClientID,
			ClientSecret: cfg.GitHub.ClientSecret,
			RedirectURL:  cfg.GitHub.RedirectURL,
			Scopes:       cfg.GitHub.Scopes,
		}, httpClient))
	}

  	if cfg.<YourProvider> != nil {
    	if err := validateConfig(auth.Provider<YourProvider>, cfg.<YourProvider>); err != nil {
		return nil, err
		}
    	reg.Register(<your_provider>.New(<your_provider>.Config{
		ClientID:     cfg.<YourProvider>.ClientID,
		ClientSecret: cfg.<YourProvider>.ClientSecret,
		RedirectURL:  cfg.<YourProvider>.RedirectURL,
		Scopes:       cfg.<YourProvider>.Scopes,
		}, httpClient))
  	}

	if len(reg.IDs()) == 0 {
		return nil, fmt.Errorf("oauth: no providers configured")
	}

	return reg, nil
}

func validateConfig(id auth.ProviderID, cfg *ProviderConfig) error {
	if cfg.ClientID == "" {
		return fmt.Errorf("oauth: %s: client_id is required", id)
	}
	if cfg.RedirectURL == "" {
		return fmt.Errorf("oauth: %s: redirect_url is required", id)
	}
	// ClientSecret is intentionally NOT validated — some deployments use
	// PKCE-only public clients with no secret.
	return nil
}

```

---

## License

MIT — see [LICENSE](LICENSE) for details.

---

## Acknowledgements

- [`golang.org/x/oauth2`](https://pkg.go.dev/golang.org/x/oauth2) — the OAuth 2.0 client library.
- [Google Identity Platform](https://developers.google.com/identity/protocols/oauth2) — Google OAuth documentation.
- [GitHub OAuth Apps](https://docs.github.com/en/apps/oauth-apps) — GitHub OAuth documentation.
- [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/) — tracing and metrics SDK.

---

_Last Updated on 11th October 2026_