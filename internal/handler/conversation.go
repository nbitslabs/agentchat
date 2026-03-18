package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/nbitslabs/agentchat/internal/database"
	"github.com/nbitslabs/agentchat/internal/middleware"
)

type ConversationHandler struct {
	queries *database.Queries
}

func NewConversationHandler(q *database.Queries) *ConversationHandler {
	return &ConversationHandler{queries: q}
}

func (h *ConversationHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	agentID := middleware.AgentIDFromContext(r.Context())
	if agentID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	partnerID := r.PathValue("partner_id")
	if partnerID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "partner_id is required")
		return
	}

	limit := int32(50)
	offset := int32(0)
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = int32(v)
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = int32(v)
		}
	}

	messages, err := h.queries.GetConversationHistory(r.Context(), database.GetConversationHistoryParams{
		SenderID:    agentID,
		RecipientID: partnerID,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to retrieve history")
		return
	}

	total, err := h.queries.CountConversationMessages(r.Context(), database.CountConversationMessagesParams{
		SenderID:    agentID,
		RecipientID: partnerID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to count messages")
		return
	}

	writeJSON(w, http.StatusOK, registerResponse{
		Success: true,
		Data: map[string]interface{}{
			"messages": messages,
			"total":    total,
		},
	})
}

func (h *ConversationHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	agentID := middleware.AgentIDFromContext(r.Context())
	if agentID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	rows, err := h.queries.GetConversations(r.Context(), agentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list conversations")
		return
	}

	conversations := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		partnerID := fmt.Sprintf("%v", row.PartnerID)

		// Extract preview from content
		preview := ""
		var contentMap map[string]string
		if err := json.Unmarshal(row.Content, &contentMap); err == nil {
			preview = contentMap["text"]
			if len(preview) > 100 {
				preview = preview[:100]
			}
		}

		conv := map[string]interface{}{
			"partner_id":      partnerID,
			"last_message_at": row.CreatedAt,
			"preview":         preview,
		}

		// Try to get username for partner
		agent, err := h.queries.GetAgentByID(r.Context(), partnerID)
		if err == nil && agent.Username.Valid && agent.UsernameStatus == "approved" {
			conv["username"] = agent.Username.String
		}

		conversations = append(conversations, conv)
	}

	writeJSON(w, http.StatusOK, registerResponse{
		Success: true,
		Data: map[string]interface{}{
			"conversations": conversations,
		},
	})
}

func (h *ConversationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	agentID := middleware.AgentIDFromContext(r.Context())
	if agentID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	var req struct {
		MessageID string `json:"message_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	if req.MessageID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "message_id is required")
		return
	}

	// Validate that the agent is the recipient
	msg, err := h.queries.GetMessageByID(r.Context(), req.MessageID)
	if err != nil {
		writeError(w, http.StatusNotFound, "MESSAGE_NOT_FOUND", "message not found")
		return
	}

	if msg.RecipientID != agentID {
		writeError(w, http.StatusForbidden, "UNAUTHORIZED", "only the recipient can mark messages as read")
		return
	}

	err = h.queries.MarkMessageReadByRecipient(r.Context(), database.MarkMessageReadByRecipientParams{
		MessageID:   req.MessageID,
		RecipientID: agentID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to mark message as read")
		return
	}

	writeJSON(w, http.StatusOK, registerResponse{Success: true})
}
