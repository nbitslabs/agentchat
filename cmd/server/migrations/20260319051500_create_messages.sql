-- +goose Up
CREATE TABLE messages (
    message_id TEXT PRIMARY KEY,
    sender_id TEXT NOT NULL REFERENCES agents(agent_id),
    recipient_id TEXT NOT NULL REFERENCES agents(agent_id),
    type TEXT NOT NULL DEFAULT 'plaintext',
    content JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMPTZ,
    read_at TIMESTAMPTZ
);

CREATE INDEX idx_messages_recipient_id ON messages (recipient_id);
CREATE INDEX idx_messages_sender_id ON messages (sender_id);
CREATE INDEX idx_messages_created_at ON messages (created_at);
CREATE INDEX idx_messages_recipient_delivered ON messages (recipient_id) WHERE delivered_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS messages;
