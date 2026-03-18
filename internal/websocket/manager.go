package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	ws "github.com/gorilla/websocket"
	"github.com/nbitslabs/agentchat/internal/database"
	"github.com/redis/go-redis/v9"
)

var upgrader = ws.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Frame is the WebSocket message format.
type Frame struct {
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp string          `json:"timestamp,omitempty"`
}

// Manager manages WebSocket connections and real-time message delivery.
type Manager struct {
	queries   *database.Queries
	redis     *redis.Client
	jwtSecret []byte

	mu    sync.RWMutex
	conns map[string]map[string]*ws.Conn // agentID -> connectionID -> conn
}

func NewManager(q *database.Queries, rdb *redis.Client, jwtSecret []byte) *Manager {
	return &Manager{
		queries:   q,
		redis:     rdb,
		jwtSecret: jwtSecret,
		conns:     make(map[string]map[string]*ws.Conn),
	}
}

// DeliverMessage delivers a message to all active connections for a recipient.
// This is the function passed to MessageHandler as the DeliveryFunc.
func (m *Manager) DeliverMessage(recipientID string, msg database.Message) {
	m.mu.RLock()
	connections, ok := m.conns[recipientID]
	if !ok || len(connections) == 0 {
		m.mu.RUnlock()
		return
	}

	// Copy connections to avoid holding lock during writes
	conns := make([]*ws.Conn, 0, len(connections))
	for _, c := range connections {
		conns = append(conns, c)
	}
	m.mu.RUnlock()

	payload, _ := json.Marshal(msg)
	frame := Frame{
		Type:      "message",
		Payload:   payload,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	frameBytes, _ := json.Marshal(frame)

	delivered := false
	for _, c := range conns {
		if err := c.WriteMessage(ws.TextMessage, frameBytes); err == nil {
			delivered = true
		}
	}

	if delivered {
		m.queries.MarkMessageDelivered(context.Background(), msg.MessageID)
	}
}

// HandleUpgrade handles WebSocket upgrade requests.
func (m *Manager) HandleUpgrade(w http.ResponseWriter, r *http.Request) {
	// Authenticate via Authorization header or token query param
	agentID, err := m.authenticate(r)
	if err != nil {
		http.Error(w, `{"error":"WEBSOCKET_AUTH_FAILED"}`, http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}

	connID := uuid.New().String()
	m.registerConnection(r.Context(), agentID, connID, conn)

	go m.handleConnection(agentID, connID, conn)
}

func (m *Manager) authenticate(r *http.Request) (string, error) {
	tokenStr := ""
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
	} else {
		tokenStr = r.URL.Query().Get("token")
	}

	if tokenStr == "" {
		return "", errors.New("no token provided")
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return "", errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid claims")
	}

	agentID, _ := claims["sub"].(string)
	if agentID == "" {
		return "", errors.New("missing agent_id in token")
	}

	return agentID, nil
}

func (m *Manager) registerConnection(ctx context.Context, agentID, connID string, conn *ws.Conn) {
	m.mu.Lock()
	if m.conns[agentID] == nil {
		m.conns[agentID] = make(map[string]*ws.Conn)
	}
	m.conns[agentID][connID] = conn
	m.mu.Unlock()

	// Register in Redis
	metadata, _ := json.Marshal(map[string]string{
		"connection_id": connID,
		"agent_id":      agentID,
		"connected_at":  time.Now().UTC().Format(time.RFC3339),
	})
	m.redis.Set(ctx, "ws:agent:"+agentID+":"+connID, string(metadata), 30*time.Minute)
	m.redis.SAdd(ctx, "ws:connections:"+agentID, connID)
	m.redis.Expire(ctx, "ws:connections:"+agentID, 30*time.Minute)
}

func (m *Manager) unregisterConnection(agentID, connID string) {
	m.mu.Lock()
	if conns, ok := m.conns[agentID]; ok {
		delete(conns, connID)
		if len(conns) == 0 {
			delete(m.conns, agentID)
		}
	}
	m.mu.Unlock()

	ctx := context.Background()
	m.redis.Del(ctx, "ws:agent:"+agentID+":"+connID)
	m.redis.SRem(ctx, "ws:connections:"+agentID, connID)
}

func (m *Manager) handleConnection(agentID, connID string, conn *ws.Conn) {
	defer func() {
		m.unregisterConnection(agentID, connID)
		conn.Close()
	}()

	// Deliver pending messages
	m.deliverPendingMessages(agentID, conn)

	// Read loop
	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var frame Frame
		if err := json.Unmarshal(msgBytes, &frame); err != nil {
			continue
		}

		switch frame.Type {
		case "mark_read":
			var payload struct {
				MessageID string `json:"message_id"`
			}
			if err := json.Unmarshal(frame.Payload, &payload); err == nil && payload.MessageID != "" {
				m.queries.MarkMessageRead(context.Background(), payload.MessageID)
			}
		}
	}
}

func (m *Manager) deliverPendingMessages(agentID string, conn *ws.Conn) {
	ctx := context.Background()
	messages, err := m.queries.GetUndeliveredMessages(ctx, agentID)
	if err != nil {
		log.Printf("failed to get undelivered messages for %s: %v", agentID, err)
		return
	}

	for _, msg := range messages {
		payload, _ := json.Marshal(msg)
		frame := Frame{
			Type:      "message",
			Payload:   payload,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		frameBytes, _ := json.Marshal(frame)

		if err := conn.WriteMessage(ws.TextMessage, frameBytes); err != nil {
			log.Printf("failed to deliver pending message %s: %v", msg.MessageID, err)
			return
		}

		m.queries.MarkMessageDelivered(ctx, msg.MessageID)
	}
}
