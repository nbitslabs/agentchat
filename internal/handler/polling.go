package handler

import (
	"net/http"

	"github.com/nbitslabs/agentchat/internal/database"
	"github.com/nbitslabs/agentchat/internal/middleware"
)

type PollingHandler struct {
	queries *database.Queries
}

func NewPollingHandler(q *database.Queries) *PollingHandler {
	return &PollingHandler{queries: q}
}

func (h *PollingHandler) Poll(w http.ResponseWriter, r *http.Request) {
	agentID := middleware.AgentIDFromContext(r.Context())
	if agentID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	messages, err := h.queries.GetUndeliveredMessages(r.Context(), agentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to retrieve messages")
		return
	}

	// Mark all returned messages as delivered
	for _, msg := range messages {
		h.queries.MarkMessageDelivered(r.Context(), msg.MessageID)
	}

	if messages == nil {
		messages = []database.Message{}
	}

	writeJSON(w, http.StatusOK, registerResponse{
		Success: true,
		Data: map[string]interface{}{
			"messages": messages,
		},
	})
}
