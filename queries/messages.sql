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

-- name: GetConversationHistory :many
SELECT * FROM messages
WHERE (sender_id = $1 AND recipient_id = $2)
   OR (sender_id = $2 AND recipient_id = $1)
ORDER BY created_at ASC
LIMIT $3 OFFSET $4;

-- name: CountConversationMessages :one
SELECT COUNT(*) FROM messages
WHERE (sender_id = $1 AND recipient_id = $2)
   OR (sender_id = $2 AND recipient_id = $1);

-- name: GetConversations :many
SELECT DISTINCT ON (partner_id)
    partner_id,
    message_id,
    sender_id,
    recipient_id,
    type,
    content,
    created_at,
    delivered_at,
    read_at
FROM (
    SELECT
        CASE WHEN sender_id = $1 THEN recipient_id ELSE sender_id END AS partner_id,
        message_id, sender_id, recipient_id, type, content, created_at, delivered_at, read_at
    FROM messages
    WHERE sender_id = $1 OR recipient_id = $1
) sub
ORDER BY partner_id, created_at DESC;

-- name: MarkMessageReadByRecipient :exec
UPDATE messages SET read_at = NOW()
WHERE message_id = $1 AND recipient_id = $2 AND read_at IS NULL;

-- name: GetAgentByApprovedUsername :one
SELECT * FROM agents WHERE LOWER(username) = LOWER($1) AND username_status = 'approved';
