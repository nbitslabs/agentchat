-- name: CreateAgent :one
INSERT INTO agents (agent_id, root_public_key)
VALUES ($1, $2)
RETURNING *;

-- name: GetAgentByID :one
SELECT * FROM agents WHERE agent_id = $1;

-- name: GetAgentByPublicKey :one
SELECT * FROM agents WHERE root_public_key = $1;

-- name: GetAgentByUsernameLower :one
SELECT * FROM agents WHERE LOWER(username) = LOWER($1);

-- name: ClaimUsername :one
UPDATE agents
SET username = $2, username_status = 'pending'
WHERE agent_id = $1 AND (username IS NULL OR username_status = 'none')
RETURNING *;
