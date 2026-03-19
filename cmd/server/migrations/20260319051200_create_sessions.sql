-- +goose Up
CREATE TABLE sessions (
    session_id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL REFERENCES agents(agent_id),
    session_public_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_sessions_agent_id ON sessions (agent_id);
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);

-- +goose Down
DROP TABLE IF EXISTS sessions;
