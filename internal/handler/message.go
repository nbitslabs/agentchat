package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/nbitslabs/agentchat/internal/database"
	"github.com/nbitslabs/agentchat/internal/middleware"
	"github.com/redis/go-redis/v9"
)

const (
	maxContentSize    = 10 * 1024 * 1024 // 10MB
	rateLimitPerMin   = 100
	rateLimitWindowSec = 60
)

var agentIDRegex = regexp.MustCompile(`^agnt_[1-9A-HJ-NP-Za-km-z]{20,40}$`)

// DeliveryFunc is called when a recipient is online to deliver a message in real-time.
// This will be set by the WebSocket infrastructure (WO-5).
type DeliveryFunc func(recipientID string, msg database.Message)

type MessageHandler struct {
	queries  *database.Queries
	redis    *redis.Client
	delivery DeliveryFunc
}

func NewMessageHandler(q *database.Queries, rdb *redis.Client, delivery DeliveryFunc) *MessageHandler {
	return &MessageHandler{queries: q, redis: rdb, delivery: delivery}
}

type sendMessageRequest struct {
	Recipient string `json:"recipient"`
	Content   string `json:"content"`
}

func (h *MessageHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	senderID := middleware.AgentIDFromContext(r.Context())
	if senderID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	if req.Recipient == "" || req.Content == "" {
		writeError(w, http.StatusBadRequest, "INVALID_CONTENT", "recipient and content are required")
		return
	}

	// Check content size
	if len(req.Content) > maxContentSize {
		writeError(w, http.StatusRequestEntityTooLarge, "MESSAGE_TOO_LARGE", "content exceeds 10MB limit")
		return
	}

	// Rate limit check
	if err := h.checkRateLimit(r, senderID); err != nil {
		retryAfter := fmt.Sprintf("%d", rateLimitWindowSec)
		w.Header().Set("Retry-After", retryAfter)
		writeError(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "exceeded 100 messages per minute")
		return
	}

	// Resolve recipient
	recipientID, err := h.resolveRecipient(r, req.Recipient)
	if err != nil {
		writeError(w, http.StatusNotFound, "RECIPIENT_NOT_FOUND", "recipient not found")
		return
	}

	// Create message envelope
	messageID := uuid.New().String()
	contentJSON, _ := json.Marshal(map[string]string{"text": req.Content})

	msg, err := h.queries.CreateMessage(r.Context(), database.CreateMessageParams{
		MessageID:   messageID,
		SenderID:    senderID,
		RecipientID: recipientID,
		Type:        "plaintext",
		Content:     contentJSON,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to persist message")
		return
	}

	// Attempt real-time delivery if handler is set
	if h.delivery != nil {
		h.delivery(recipientID, msg)
	}

	writeJSON(w, http.StatusCreated, registerResponse{
		Success: true,
		Data: map[string]string{
			"message_id": messageID,
			"timestamp":  msg.CreatedAt.Format(time.RFC3339),
		},
	})
}

func (h *MessageHandler) resolveRecipient(r *http.Request, recipient string) (string, error) {
	if agentIDRegex.MatchString(recipient) {
		// Direct agent ID lookup
		_, err := h.queries.GetAgentByID(r.Context(), recipient)
		if err != nil {
			return "", err
		}
		return recipient, nil
	}

	// Username lookup (approved only)
	agent, err := h.queries.GetAgentByApprovedUsername(r.Context(), recipient)
	if err != nil {
		return "", err
	}
	return agent.AgentID, nil
}

func (h *MessageHandler) checkRateLimit(r *http.Request, agentID string) error {
	window := time.Now().Unix() / rateLimitWindowSec
	key := fmt.Sprintf("ratelimit:agent:%s:%d", agentID, window)

	count, err := h.redis.Incr(r.Context(), key).Result()
	if err != nil {
		// If Redis is down, allow the request
		return nil
	}

	if count == 1 {
		h.redis.Expire(r.Context(), key, 2*time.Minute)
	}

	if count > rateLimitPerMin {
		return errors.New("rate limit exceeded")
	}

	return nil
}

