package server

import (
	"net/http"

	"github.com/nbitslabs/agentchat/internal/database"
	"github.com/nbitslabs/agentchat/internal/handler"
	"github.com/nbitslabs/agentchat/internal/middleware"
)

func NewRouter(queries *database.Queries) http.Handler {
	mux := http.NewServeMux()

	reg := handler.NewRegistrationHandler(queries)
	usr := handler.NewUsernameHandler(queries)

	mux.HandleFunc("POST /api/v1/agents/register", reg.Register)
	mux.Handle("POST /api/v1/agents/username/claim", middleware.RequireAuth(http.HandlerFunc(usr.ClaimUsername)))

	return mux
}
