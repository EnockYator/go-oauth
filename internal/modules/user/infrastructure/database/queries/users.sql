-- name: UpsertUser :one
-- The one query the OAuth callback actually needs. On first login it inserts;
-- on every subsequent login it refreshes the mutable profile fields and returns
-- the row. provider_subject is the conflict target — email is NOT, because
-- emails change and a collision on email must not merge two identities.
INSERT INTO users (
    email,
    name,
    avatar_url,
    provider,
    provider_subject
) VALUES (
    $1, $2, $3, $4, $5
)
ON CONFLICT (provider, provider_subject)
DO UPDATE SET
    email      = EXCLUDED.email,
    name       = EXCLUDED.name,
    avatar_url = EXCLUDED.avatar_url,
    updated_at = NOW()
RETURNING *;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = $1;

-- name: GetUserByProviderSubject :one
-- Exact lookup on the identity key. Used when you already know the provider
-- (e.g. re-authenticating a session whose provider you stored).
SELECT *
FROM users
WHERE provider = $1
  AND provider_subject = $2;

-- name: GetUserByEmail :one
-- Case-insensitive, matches users_email_idx. NOTE: email is unique, but a user
-- may have one row per provider, so this can return multiple rows if you allow
-- the same email across providers. If you do, switch to :many and reconcile.
SELECT *
FROM users
WHERE LOWER(email) = LOWER($1);

-- name: GetUserByEmailAndProvider :one
-- Use this instead of GetUserByEmail when you want a single-identity lookup.
SELECT *
FROM users
WHERE LOWER(email) = LOWER($1)
  AND provider = $2;

-- name: ListUsers :many
-- Admin listing. Keyset pagination on id is safer than OFFSET at scale.
SELECT *
FROM users
ORDER BY created_at DESC, id
LIMIT $1 OFFSET $2;

-- name: UpdateUserProfile :one
-- Manual profile edit (name / avatar). Deliberately does NOT touch email or
-- provider fields — those come from the IdP and would be overwritten on the
-- next login anyway.
UPDATE users
SET name       = $2,
    avatar_url = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CountUsers :one
SELECT count(*) FROM users;

-- name: DeleteUser :execrows
-- Hard delete. Sessions cascade via the FK. Prefer soft-delete / anonymize
-- if you have audit or compliance requirements.
DELETE FROM users
WHERE id = $1;