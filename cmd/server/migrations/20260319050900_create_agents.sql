-- +goose Up
CREATE TABLE agents (
    agent_id TEXT PRIMARY KEY,
    root_public_key TEXT NOT NULL UNIQUE,
    username TEXT,
    username_status TEXT NOT NULL DEFAULT 'none' CHECK (username_status IN ('none', 'pending', 'approved')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_agents_username_lower ON agents (LOWER(username)) WHERE username IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS agents;
