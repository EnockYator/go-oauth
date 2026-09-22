-- name: CreateSession :one
INSERT INTO sessions (
    id_hash,
    user_id,
    expires_at,
    metadata
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: GetSession :one
-- Valid = not revoked AND not expired. Use after hashing the cookie.
SELECT *
FROM sessions
WHERE id_hash = $1
  AND revoked_at IS NULL
  AND expires_at > NOW();

-- name: GetSessionWithUser :one
-- Same validity rules, joined to the user so middleware can skip a second query.
SELECT
    s.id_hash,
    s.user_id,
    s.created_at,
    s.expires_at,
    s.revoked_at,
    s.last_seen,
    s.metadata,
    u.email,
    u.name,
    u.avatar_url,
    u.provider,
    u.provider_subject
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.id_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > NOW();

-- name: TouchSession :execrows
-- Sliding renewal: bump last_seen and push expires_at forward.
-- Call on authenticated requests, throttled (e.g. only if last_seen < now() - 1m).
UPDATE sessions
SET last_seen  = NOW(),
    expires_at = NOW() + $2::interval
WHERE id_hash = $1
  AND revoked_at IS NULL
  AND expires_at > NOW();

-- name: RevokeSession :execrows
-- Logout for a single session. Soft revoke — keeps the audit trail in metadata.
UPDATE sessions
SET revoked_at = NOW()
WHERE id_hash = $1
  AND revoked_at IS NULL;

-- name: RevokeUserSessions :execrows
-- "Log out everywhere" / revoke all on password or privilege change.
UPDATE sessions
SET revoked_at = NOW()
WHERE user_id = $1
  AND revoked_at IS NULL;

-- name: RevokeUserSessionsExcept :execrows
-- Rotate on login: kill all other sessions, keep the one just created.
UPDATE sessions
SET revoked_at = NOW()
WHERE user_id = $1
  AND id_hash <> $2
  AND revoked_at IS NULL;

-- name: ListUserSessions :many
-- "Active sessions" UI. Returns live sessions only, newest first.
SELECT
    id_hash,
    created_at,
    expires_at,
    last_seen,
    metadata
FROM sessions
WHERE user_id = $1
  AND revoked_at IS NULL
  AND expires_at > NOW()
ORDER BY last_seen DESC;

-- name: CountUserSessions :one
SELECT count(*)
FROM sessions
WHERE user_id = $1
  AND revoked_at IS NULL
  AND expires_at > NOW();

-- name: DeleteExpiredSessions :execrows
-- Cleanup job. Hard-deletes rows that are expired, or revoked more than
-- $1::interval ago, so metadata doesn't accumulate forever.
DELETE FROM sessions
WHERE expires_at <= NOW()
   OR (revoked_at IS NOT NULL AND revoked_at <= NOW() - $1::interval);