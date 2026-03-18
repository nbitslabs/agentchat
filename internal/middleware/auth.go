package middleware

import (
	"context"
	"net/http"
)

type contextKey string

const agentIDKey contextKey = "agent_id"

// AgentIDFromContext extracts the authenticated agent ID from the request context.
func AgentIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(agentIDKey).(string)
	return v
}

// WithAgentID returns a new context with the agent ID set.
func WithAgentID(ctx context.Context, agentID string) context.Context {
	return context.WithValue(ctx, agentIDKey, agentID)
}

// RequireAuth is a placeholder middleware that will be replaced by JWT-based
// authentication in WO-2. For now, it reads agent_id from the X-Agent-ID header.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agentID := r.Header.Get("X-Agent-ID")
		if agentID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"success":false,"error":{"code":"UNAUTHORIZED","message":"authentication required"}}`))
			return
		}
		ctx := WithAgentID(r.Context(), agentID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
