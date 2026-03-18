package server

import (
	"net/http"

	"github.com/nbitslabs/agentchat/internal/database"
	"github.com/nbitslabs/agentchat/internal/handler"
	"github.com/nbitslabs/agentchat/internal/middleware"
	"github.com/nbitslabs/agentchat/internal/websocket"
	"github.com/redis/go-redis/v9"
)

func NewRouter(queries *database.Queries, rdb *redis.Client, jwtSecret []byte) http.Handler {
	mux := http.NewServeMux()

	wsMgr := websocket.NewManager(queries, rdb, jwtSecret)

	reg := handler.NewRegistrationHandler(queries)
	usr := handler.NewUsernameHandler(queries)
	sess := handler.NewSessionHandler(queries, rdb, jwtSecret)
	prof := handler.NewProfileHandler(queries)
	msg := handler.NewMessageHandler(queries, rdb, wsMgr.DeliverMessage)

	authMiddleware := middleware.RequireAuth(jwtSecret, rdb, queries)

	// Public endpoints
	mux.HandleFunc("POST /api/v1/agents/register", reg.Register)
	mux.HandleFunc("POST /api/v1/sessions/create", sess.CreateSession)

	// WebSocket endpoint (auth handled internally)
	mux.HandleFunc("GET /api/v1/ws", wsMgr.HandleUpgrade)

	conv := handler.NewConversationHandler(queries)

	// Authenticated endpoints
	mux.Handle("POST /api/v1/agents/username/claim", authMiddleware(http.HandlerFunc(usr.ClaimUsername)))
	mux.Handle("GET /api/v1/agents/me", authMiddleware(http.HandlerFunc(prof.GetMe)))
	mux.Handle("POST /api/v1/messages/send", authMiddleware(http.HandlerFunc(msg.SendMessage)))
	mux.Handle("GET /api/v1/messages/history/{partner_id}", authMiddleware(http.HandlerFunc(conv.GetHistory)))
	mux.Handle("GET /api/v1/messages/conversations", authMiddleware(http.HandlerFunc(conv.ListConversations)))
	mux.Handle("POST /api/v1/messages/mark-read", authMiddleware(http.HandlerFunc(conv.MarkRead)))

	return mux
}
