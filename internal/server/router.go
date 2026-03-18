package server

import (
	"net/http"

	"github.com/nbitslabs/agentchat/internal/database"
	"github.com/nbitslabs/agentchat/internal/handler"
	"github.com/nbitslabs/agentchat/internal/middleware"
	"github.com/redis/go-redis/v9"
)

func NewRouter(queries *database.Queries, rdb *redis.Client, jwtSecret []byte) http.Handler {
	mux := http.NewServeMux()

	reg := handler.NewRegistrationHandler(queries)
	usr := handler.NewUsernameHandler(queries)
	sess := handler.NewSessionHandler(queries, rdb, jwtSecret)

	authMiddleware := middleware.RequireAuth(jwtSecret, rdb, queries)

	// Public endpoints
	mux.HandleFunc("POST /api/v1/agents/register", reg.Register)
	mux.HandleFunc("POST /api/v1/sessions/create", sess.CreateSession)

	prof := handler.NewProfileHandler(queries)
	msg := handler.NewMessageHandler(queries, rdb, nil) // delivery func set by WO-5

	// Authenticated endpoints
	mux.Handle("POST /api/v1/agents/username/claim", authMiddleware(http.HandlerFunc(usr.ClaimUsername)))
	mux.Handle("GET /api/v1/agents/me", authMiddleware(http.HandlerFunc(prof.GetMe)))
	mux.Handle("POST /api/v1/messages/send", authMiddleware(http.HandlerFunc(msg.SendMessage)))

	return mux
}
