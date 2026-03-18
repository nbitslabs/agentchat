package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"github.com/nbitslabs/agentchat/internal/database"
	"github.com/nbitslabs/agentchat/internal/middleware"
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

type UsernameHandler struct {
	queries *database.Queries
}

func NewUsernameHandler(q *database.Queries) *UsernameHandler {
	return &UsernameHandler{queries: q}
}

type claimUsernameRequest struct {
	Username string `json:"username"`
}

func (h *UsernameHandler) ClaimUsername(w http.ResponseWriter, r *http.Request) {
	agentID := middleware.AgentIDFromContext(r.Context())
	if agentID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	var req claimUsernameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	if !usernameRegex.MatchString(req.Username) {
		writeError(w, http.StatusBadRequest, "INVALID_USERNAME_FORMAT", "username must be 3-32 characters, alphanumeric, hyphens, or underscores")
		return
	}

	// Check case-insensitive uniqueness
	_, err := h.queries.GetAgentByUsernameLower(r.Context(), req.Username)
	if err == nil {
		writeError(w, http.StatusConflict, "USERNAME_TAKEN", "this username is already taken or pending")
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	_, err = h.queries.ClaimUsername(r.Context(), database.ClaimUsernameParams{
		AgentID:  agentID,
		Username: sql.NullString{String: req.Username, Valid: true},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "agent not found or username already claimed")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to claim username")
		return
	}

	writeJSON(w, http.StatusOK, registerResponse{
		Success: true,
		Data:    map[string]string{"status": "pending"},
	})
}
