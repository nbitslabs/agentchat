package handler

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/nbitslabs/agentchat/internal/database"
	"github.com/nbitslabs/agentchat/internal/middleware"
)

type ProfileHandler struct {
	queries *database.Queries
}

func NewProfileHandler(q *database.Queries) *ProfileHandler {
	return &ProfileHandler{queries: q}
}

func (h *ProfileHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	agentID := middleware.AgentIDFromContext(r.Context())
	if agentID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	agent, err := h.queries.GetAgentByID(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "AGENT_NOT_FOUND", "agent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	data := map[string]interface{}{
		"agent_id":        agent.AgentID,
		"root_public_key": agent.RootPublicKey,
		"username_status": agent.UsernameStatus,
		"created_at":      agent.CreatedAt,
	}

	if agent.Username.Valid && agent.UsernameStatus == "approved" {
		data["username"] = agent.Username.String
	} else if agent.Username.Valid && agent.UsernameStatus == "pending" {
		data["username"] = agent.Username.String
	}

	writeJSON(w, http.StatusOK, registerResponse{
		Success: true,
		Data:    data,
	})
}
