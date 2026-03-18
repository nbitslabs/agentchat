-- name: CreateMessage :one
INSERT INTO messages (message_id, sender_id, recipient_id, type, content)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetMessageByID :one
SELECT * FROM messages WHERE message_id = $1;

-- name: MarkMessageDelivered :exec
UPDATE messages SET delivered_at = NOW() WHERE message_id = $1 AND delivered_at IS NULL;

-- name: GetUndeliveredMessages :many
SELECT * FROM messages WHERE recipient_id = $1 AND delivered_at IS NULL ORDER BY created_at ASC;

-- name: MarkMessageRead :exec
UPDATE messages SET read_at = NOW() WHERE message_id = $1 AND read_at IS NULL;

-- name: GetAgentByApprovedUsername :one
SELECT * FROM agents WHERE LOWER(username) = LOWER($1) AND username_status = 'approved';
