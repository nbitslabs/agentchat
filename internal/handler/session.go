package handler

import (
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/nbitslabs/agentchat/internal/crypto"
	"github.com/nbitslabs/agentchat/internal/database"
	"github.com/redis/go-redis/v9"
)

const sessionDuration = 6 * time.Hour

type SessionHandler struct {
	queries   *database.Queries
	redis     *redis.Client
	jwtSecret []byte
}

func NewSessionHandler(q *database.Queries, rdb *redis.Client, jwtSecret []byte) *SessionHandler {
	return &SessionHandler{queries: q, redis: rdb, jwtSecret: jwtSecret}
}

type createSessionRequest struct {
	AgentID          string `json:"agent_id"`
	SessionPublicKey string `json:"session_public_key"`
	Signature        string `json:"signature"`
}

func (h *SessionHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	if req.AgentID == "" || req.SessionPublicKey == "" || req.Signature == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "agent_id, session_public_key, and signature are required")
		return
	}

	// Decode session public key
	sessionPubKeyBytes, err := base64.StdEncoding.DecodeString(req.SessionPublicKey)
	if err != nil || len(sessionPubKeyBytes) != ed25519.PublicKeySize {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid session public key")
		return
	}

	// Decode signature
	sigBytes, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid signature encoding")
		return
	}

	// Look up the agent to get root public key
	agent, err := h.queries.GetAgentByID(r.Context(), req.AgentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "AGENT_NOT_FOUND", "agent not registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	// Decode the stored root public key
	rootPubKey, err := crypto.DecodePublicKey(agent.RootPublicKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	// Verify: signature covers the session public key bytes, signed by root private key
	if !ed25519.Verify(rootPubKey, sessionPubKeyBytes, sigBytes) {
		writeError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "signature verification failed")
		return
	}

	// Create session
	sessionID := uuid.New().String()
	now := time.Now().UTC()
	expiresAt := now.Add(sessionDuration)

	_, err = h.queries.CreateSession(r.Context(), database.CreateSessionParams{
		SessionID:        sessionID,
		AgentID:          req.AgentID,
		SessionPublicKey: req.SessionPublicKey,
		ExpiresAt:        expiresAt,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create session")
		return
	}

	// Cache in Redis
	cacheVal, _ := json.Marshal(map[string]string{
		"agent_id":   req.AgentID,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
	h.redis.Set(r.Context(), "session:"+sessionID, string(cacheVal), sessionDuration)

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": req.AgentID,
		"sid": sessionID,
		"exp": expiresAt.Unix(),
		"iat": now.Unix(),
	})
	tokenString, err := token.SignedString(h.jwtSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate token")
		return
	}

	writeJSON(w, http.StatusCreated, registerResponse{
		Success: true,
		Data: map[string]string{
			"session_token": tokenString,
			"expires_at":    expiresAt.Format(time.RFC3339),
		},
	})
}
