## Architecture

### Architecture Diagram

```
                          ┌──────────────────────────┐
                          │     Google / GitHub      │
                          │                          │
                          │      OAuth 2.0 / OIDC    │
                          └────────────▲─────────────┘
                                       │
                                       | 1. callback
                                       | 2. authorize
                                       │
                                       │
┌──────────────────┐         ┌─────────┴──────────┐
│     Browser      │────────▶│     Go OAuth API   │
│                  │         │      (This App)    │
│  Cookie:         │◀────────│                    │
│  sid=<sessionID> |         │                    │
└──────────────────┘         └─────────┬──────────┘
                                       │
                                       │
                                       │ 3. exchange code
                                       │ 4. fetch identity
                                       │ 5. create/load session
                                       ▼
                      ┌──────────────────────────────────────┐
                      │    PostgreSQL  (session lookup)      |
                      │                                      │
                      │     users               sessions     │              ┌─────────────────────────────┐
                      │  ──────────────      ────────────    │              │       Session Cleanup       │
                      │  id                  id_hash         │              │                             |
                      │  email(unique)       user_id         │              │  Periodic background job    |
                      │  name                created_at      │─────────────▶|                             |
                      │  avatar_url          expires_at      │              |  DELETE expired/revoked     |
                      │  provider            last_seen_at    │              |  sessions                   |
                      │  provider_subject    revoked_at      │              └─────────────────────────────┘
                      │  created_at          metadata        │
                      │  updated_at                          │                    
                      └──────────────────────────────────────┘
```

The service performs the full **Authorization Code flow** server-side. The browser never sees access tokens; it only receives a signed session cookie.

### Authentication flow

#### 1. Login flow (/auth/{provider} → /auth/{provider}/callback)

```text
┌─────────┐               ┌──────────────┐              ┌───────────────┐
│ Browser │               │ Go OAuth API │              │ Google/GitHub │
└────┬────┘               └──────┬───────┘              └───────┬───────┘
     │                           │                              │
     │ GET /auth/{provider}      │                              │
     │──────────────────────────>│                              │
     │                           │                              │
     │                           │ Generate state               │
     │                           │ Generate authorization URL   │
     │                           │                              │
     │ 302 Location: provider    │                              │
     │<──────────────────────────│                              │
     │                           │                              │
     │ GET authorization URL     │                              │
     │─────────────────────────────────────────────────────────>│
     │                           │                              │
     │                           │                              │
     │     User authenticates / authorizes application          │
     │                           │                              │
     │                           │                              │
     │ 302 /auth/github/callback │                              │
     │<─────────────────────────────────────────────────────────│
     │                           │                              │
     │ GET /auth/github/callback │                              │
     │ ?code=...&state=...       │                              │
     │──────────────────────────>│                              │
     │                           │                              │
     │                           │ Validate state               │
     │                           │                              │
     │                           │ POST /token                  │
     │                           │─────────────────────────────>│
     │                           │                              │
     │                           │<──── access token ───────────│
     │                           │                              │
     │                           │ GET userinfo/profile         │
     │                           │─────────────────────────────>│
     │                           │                              │
     │                           │<──── provider identity ──────│
     │                           │                              │
     │                           │                              │
     │                           │ Discard provider token       │
     │                           │                              │
     │                           │ Find/create local user       │
     │                           │                              │
     │                           │ Create local session         │
     │                           │                              │
     │                           ▼                              │
     │                    ┌───────────────┐                     │
     │                    │  PostgreSQL   │                     │
     │                    │               │                     │
     │                    │ users         │                     │
     │                    │ sessions      │                     │
     │                    └───────────────┘                     │
     │                           │                              │
     │                           │ session created              │
     │                           │                              │
     │ Set-Cookie: sid=<random>  │                              │
     │<──────────────────────────│                              │
     │                           │                              │
     │        Authenticated application session                 │
     │                           │                              │
```
   

#### 2. Authenticated request (GET /api/me)

After authentication, OAuth is no longer involved in every request.

```text
┌─────────┐               ┌──────────────┐              ┌──────────────┐
│ Browser │               │ Go OAuth API │              │  PostgreSQL  │
└────┬────┘               └──────┬───────┘              └──────┬───────┘
     │                           │                             │
     │ GET /api/me               │                             │
     │ Cookie: sid=<random>      │                             │
     │──────────────────────────>│                             │
     │                           │                             │
     │                           │ Hash session ID             │
     │                           │                             │
     │                           │ SELECT * FROM sessions      │
     │                           │    WHERE token = $1 ───────▶│
     │                           │◀───────── session row ──────│
     │                           │                             │
     │                           │ SELECT * FROM users         │
     │                           │ WHERE id = $1 ─────────────▶│
     │                           │◀───────── user row ─────────│
     │                           │                             │
     │                           │ Attach identity to context  │
     │                           │                             │
     │                           │ Execute handler             │
     │                           │                             │
     │ 200 OK { user JSON }      │                             │
     │◀──────────────────────────┤                             │
```

#### 3. Logout flow

Logout immediately revokes a session.

```text
┌─────────┐               ┌──────────────┐              ┌──────────────┐
│ Browser │               │ Go OAuth API │              │  PostgreSQL  │
└────┬────┘               └──────┬───────┘              └──────┬───────┘
     │                           │                             │
     │ POST /auth/logout         │                             │
     │ Cookie: sid=<random>      │                             │
     │──────────────────────────>│                             │
     │                           │                             │
     │                           │ Hash session ID             │
     │                           │                             │
     │                           │ UPDATE sessions             │
     │                           │ SET revoked_at = NOW()      │
     │                           │────────────────────────────>│
     │                           │                             │
     │                           │ Delete/expire cookie        │
     │                           │                             │
     │ 204 No Content            │                             │
     │ Set-Cookie: sid=deleted   │                             │
     │<──────────────────────────│                             │
```

### Minimal Schema

`migrations/001_create_extensions.sql`

```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
```

`migrations/002_create_users.sql`

```sql
CREATE TABLE IF NOT EXISTS users (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email             TEXT NOT NULL UNIQUE,
    name              TEXT NOT NULL,
    avatar_url        TEXT,
    provider          TEXT NOT NULL,
    provider_subject  TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_provider_subject_unique UNIQUE (provider, provider_subject)
);

CREATE INDEX IF NOT EXISTS users_email_idx ON users (LOWER(email));
```

The `UNIQUE (provider, provider_subject)` constraint is the account-linking key: when the same Google `sub` returns, you find the existing user instead of creating a duplicate.

`migrations/003_create_sessions.sql`

```sql
CREATE TABLE IF NOT EXISTS sessions (
    id_hash      TEXT PRIMARY KEY,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ
    last_seen    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata     JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS sessions_expires_at_idx ON sessions (expires_at);
CREATE INDEX IF NOT EXISTS sessions_user_id_idx    ON sessions (user_id);
```