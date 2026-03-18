package handler

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/nbitslabs/agentchat/internal/database"
	"github.com/redis/go-redis/v9"
)

type DiscoveryHandler struct {
	queries *database.Queries
	redis   *redis.Client
}

func NewDiscoveryHandler(q *database.Queries, rdb *redis.Client) *DiscoveryHandler {
	return &DiscoveryHandler{queries: q, redis: rdb}
}

func (h *DiscoveryHandler) Search(w http.ResponseWriter, r *http.Request) {
	if err := h.checkIPRateLimit(r); err != nil {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "too many requests")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "query parameter q is required")
		return
	}

	limit, offset := parsePagination(r)

	agents, err := h.queries.SearchAgentsByUsername(r.Context(), database.SearchAgentsByUsernameParams{
		Lower:  "%" + query + "%",
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "search failed")
		return
	}

	writeJSON(w, http.StatusOK, registerResponse{
		Success: true,
		Data: map[string]interface{}{
			"agents": formatAgentProfiles(agents),
		},
	})
}

func (h *DiscoveryHandler) LookupAgent(w http.ResponseWriter, r *http.Request) {
	if err := h.checkIPRateLimit(r); err != nil {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "too many requests")
		return
	}

	agentID := r.PathValue("id")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "agent ID is required")
		return
	}

	agent, err := h.queries.GetAgentByID(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "AGENT_NOT_FOUND", "agent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "lookup failed")
		return
	}

	profile := map[string]interface{}{
		"agent_id":        agent.AgentID,
		"root_public_key": agent.RootPublicKey,
		"fingerprint":     computeFingerprint(agent.RootPublicKey),
	}
	if agent.Username.Valid && agent.UsernameStatus == "approved" {
		profile["username"] = agent.Username.String
	}

	writeJSON(w, http.StatusOK, registerResponse{
		Success: true,
		Data:    profile,
	})
}

func (h *DiscoveryHandler) Directory(w http.ResponseWriter, r *http.Request) {
	if err := h.checkIPRateLimit(r); err != nil {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "too many requests")
		return
	}

	limit, offset := parsePagination(r)

	agents, err := h.queries.ListApprovedAgents(r.Context(), database.ListApprovedAgentsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "directory listing failed")
		return
	}

	total, err := h.queries.CountApprovedAgents(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "count failed")
		return
	}

	writeJSON(w, http.StatusOK, registerResponse{
		Success: true,
		Data: map[string]interface{}{
			"agents": formatAgentProfiles(agents),
			"total":  total,
		},
	})
}

func (h *DiscoveryHandler) checkIPRateLimit(r *http.Request) error {
	ip := r.RemoteAddr
	window := time.Now().Unix() / 60
	key := fmt.Sprintf("ratelimit:ip:%s:%d", ip, window)

	count, err := h.redis.Incr(r.Context(), key).Result()
	if err != nil {
		return nil // Allow on Redis failure
	}
	if count == 1 {
		h.redis.Expire(r.Context(), key, 2*time.Minute)
	}
	if count > 60 {
		return fmt.Errorf("rate limit exceeded")
	}
	return nil
}

func parsePagination(r *http.Request) (int32, int32) {
	limit := int32(20)
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
	return limit, offset
}

func formatAgentProfiles(agents []database.Agent) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(agents))
	for _, a := range agents {
		profile := map[string]interface{}{
			"agent_id":        a.AgentID,
			"root_public_key": a.RootPublicKey,
			"fingerprint":     computeFingerprint(a.RootPublicKey),
		}
		if a.Username.Valid && a.UsernameStatus == "approved" {
			profile["username"] = a.Username.String
		}
		result = append(result, profile)
	}
	return result
}

func computeFingerprint(pubKeyB64 string) string {
	decoded, err := base64.StdEncoding.DecodeString(pubKeyB64)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(decoded)
	fp := ""
	for i := 0; i < 16; i++ {
		if i > 0 {
			fp += ":"
		}
		fp += fmt.Sprintf("%02x", hash[i])
	}
	return fp
}
