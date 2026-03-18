package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nbitslabs/agentchat/internal/database"
	"github.com/redis/go-redis/v9"
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

// RequireAuth validates JWT bearer tokens and injects agent ID into context.
// It checks Redis cache first, falling back to PostgreSQL for session validation.
func RequireAuth(jwtSecret []byte, rdb *redis.Client, queries *database.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				writeAuthError(w, "INVALID_TOKEN", "missing or invalid authorization header")
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")

			// Parse and validate JWT
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, errors.New("unexpected signing method")
				}
				return jwtSecret, nil
			})
			if err != nil || !token.Valid {
				writeAuthError(w, "INVALID_TOKEN", "invalid or expired token")
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				writeAuthError(w, "INVALID_TOKEN", "invalid token claims")
				return
			}

			agentID, _ := claims["sub"].(string)
			sessionID, _ := claims["sid"].(string)
			if agentID == "" || sessionID == "" {
				writeAuthError(w, "INVALID_TOKEN", "missing required token claims")
				return
			}

			// Check session validity - try Redis first
			valid, err := checkSessionRedis(r.Context(), rdb, sessionID, agentID)
			if err != nil {
				// Cache miss - fall back to PostgreSQL
				valid, err = checkSessionDB(r.Context(), queries, rdb, sessionID, agentID)
				if err != nil {
					writeAuthError(w, "INVALID_TOKEN", "session validation failed")
					return
				}
			}

			if !valid {
				writeAuthError(w, "SESSION_EXPIRED", "session has expired")
				return
			}

			ctx := WithAgentID(r.Context(), agentID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func checkSessionRedis(ctx context.Context, rdb *redis.Client, sessionID, expectedAgentID string) (bool, error) {
	val, err := rdb.Get(ctx, "session:"+sessionID).Result()
	if err != nil {
		return false, err
	}

	var cached struct {
		AgentID   string `json:"agent_id"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal([]byte(val), &cached); err != nil {
		return false, err
	}

	if cached.AgentID != expectedAgentID {
		return false, nil
	}

	expiresAt, err := time.Parse(time.RFC3339, cached.ExpiresAt)
	if err != nil {
		return false, err
	}

	return time.Now().Before(expiresAt), nil
}

func checkSessionDB(ctx context.Context, queries *database.Queries, rdb *redis.Client, sessionID, expectedAgentID string) (bool, error) {
	session, err := queries.GetSessionByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	if session.AgentID != expectedAgentID {
		return false, nil
	}

	if time.Now().After(session.ExpiresAt) {
		return false, nil
	}

	// Re-cache in Redis
	ttl := time.Until(session.ExpiresAt)
	if ttl > 0 {
		cacheVal, _ := json.Marshal(map[string]string{
			"agent_id":   session.AgentID,
			"expires_at": session.ExpiresAt.Format(time.RFC3339),
		})
		rdb.Set(ctx, "session:"+sessionID, string(cacheVal), ttl)
	}

	return true, nil
}

func writeAuthError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   map[string]string{"code": code, "message": message},
	})
}
