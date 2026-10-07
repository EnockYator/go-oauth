-- name: CreateSession :one
INSERT INTO sessions (id_hash, user_id, expires_at, metadata)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetActiveSessionByIDHash :one
-- Valid = not revoked AND not expired. Used after hashing the cookie.
SELECT *
FROM sessions
WHERE id_hash = $1
  AND revoked_at IS NULL
  AND expires_at > NOW();

-- name: GetSessionByIDHash :one
-- Can be active / inactive session (expired / revoked)
SELECT * FROM sessions
WHERE id_hash = $1;


-- name: GetActiveSessionWithUserByIDHash :one
-- Valid = not revoked AND not expired
-- Joined to the user.
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

-- name: TouchSessionLastSeen :one
-- Update activity without extending the absolute session lifetime.
-- Call on authenticated requests only after a small application-level throttle.
UPDATE sessions
SET last_seen = NOW()
WHERE id_hash = $1
  AND revoked_at IS NULL
  AND expires_at > NOW()
RETURNING *;

-- name: RevokeSessionByIDHash :execrows
-- Logout for a single session. Soft revoke — keeps the audit trail in metadata.
UPDATE sessions
SET revoked_at = NOW()
WHERE id_hash = $1
  AND revoked_at IS NULL;

-- name: RevokeAllUserSessions :execrows
-- "Log out everywhere" / revoke all on password or privilege change.
UPDATE sessions
SET revoked_at = NOW()
WHERE user_id = $1
  AND revoked_at IS NULL;

-- name: RevokeUserSessionsExceptCurrent :execrows
-- Rotate on login: kill all other sessions, keep the one just created.
UPDATE sessions
SET revoked_at = NOW()
WHERE user_id = $1
  AND id_hash <> $2
  AND revoked_at IS NULL;

-- name: ListUserActiveSessions :many
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

-- name: CountActiveUserSessions :one
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
