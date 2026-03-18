package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nbitslabs/agentchat/internal/crypto"
	"github.com/nbitslabs/agentchat/internal/database"
)

type RegistrationHandler struct {
	queries *database.Queries
}

func NewRegistrationHandler(q *database.Queries) *RegistrationHandler {
	return &RegistrationHandler{queries: q}
}

type registerRequest struct {
	RootPublicKey string `json:"root_public_key"`
}

type registerResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *apiError   `json:"error,omitempty"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (h *RegistrationHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	if req.RootPublicKey == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "root_public_key is required")
		return
	}

	pubkey, err := crypto.DecodePublicKey(req.RootPublicKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	agentID := crypto.DeriveAgentID(pubkey)

	// Check if already registered
	_, err = h.queries.GetAgentByPublicKey(r.Context(), req.RootPublicKey)
	if err == nil {
		writeError(w, http.StatusConflict, "AGENT_ALREADY_REGISTERED", "this public key is already registered")
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	_, err = h.queries.CreateAgent(r.Context(), database.CreateAgentParams{
		AgentID:       agentID,
		RootPublicKey: req.RootPublicKey,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create agent")
		return
	}

	writeJSON(w, http.StatusCreated, registerResponse{
		Success: true,
		Data:    map[string]string{"agent_id": agentID},
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, registerResponse{
		Success: false,
		Error:   &apiError{Code: code, Message: message},
	})
}
