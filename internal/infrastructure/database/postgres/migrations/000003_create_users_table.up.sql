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
-- indexes
CREATE INDEX IF NOT EXISTS users_email_idx ON users (LOWER(email));

-- updated_at trigger
CREATE TRIGGER trg_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();