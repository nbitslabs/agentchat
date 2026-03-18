-- name: SearchAgentsByUsername :many
SELECT agent_id, root_public_key, username, username_status, created_at
FROM agents
WHERE username_status = 'approved'
  AND LOWER(username) LIKE LOWER($1)
ORDER BY username ASC
LIMIT $2 OFFSET $3;

-- name: ListApprovedAgents :many
SELECT agent_id, root_public_key, username, username_status, created_at
FROM agents
WHERE username_status = 'approved'
ORDER BY username ASC
LIMIT $1 OFFSET $2;

-- name: CountApprovedAgents :one
SELECT COUNT(*) FROM agents WHERE username_status = 'approved';
