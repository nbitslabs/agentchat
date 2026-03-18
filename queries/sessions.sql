-- name: CreateSession :one
INSERT INTO sessions (session_id, agent_id, session_public_key, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetSessionByID :one
SELECT * FROM sessions WHERE session_id = $1;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at < NOW() - INTERVAL '1 hour';
