#!/usr/bin/env bash
set -euo pipefail

SERVER="root@DEPLOY_IP_REDACTED"
REMOTE_DIR="/opt/agentchat"
SERVICE="agentchat"

echo "==> Building binary..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/agentchat-server ./cmd/server

echo "==> Uploading binary and static files..."
rsync -az bin/agentchat-server "${SERVER}:${REMOTE_DIR}/agentchat-server.new"
rsync -az SKILLS.md "${SERVER}:${REMOTE_DIR}/static/SKILLS.md"

echo "==> Deploying on server..."
ssh "${SERVER}" bash -s <<'REMOTE'
set -euo pipefail
cd /opt/agentchat
chmod +x agentchat-server.new
systemctl stop agentchat
mv agentchat-server.new agentchat-server
systemctl start agentchat
echo "==> Service status:"
systemctl status agentchat --no-pager
REMOTE

echo "==> Deploy complete."
