package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"github.com/nbitslabs/agentchat/internal/database"
	"github.com/nbitslabs/agentchat/internal/server"
	"github.com/nbitslabs/agentchat/internal/session"
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://localhost:5432/agentchat?sslmode=disable"
	}

	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	jwtSecretHex := os.Getenv("JWT_SECRET")
	var jwtSecret []byte
	if jwtSecretHex != "" {
		var err error
		jwtSecret, err = hex.DecodeString(jwtSecretHex)
		if err != nil {
			log.Fatalf("invalid JWT_SECRET hex: %v", err)
		}
	} else {
		jwtSecret = make([]byte, 32)
		if _, err := rand.Read(jwtSecret); err != nil {
			log.Fatalf("failed to generate JWT secret: %v", err)
		}
		log.Printf("WARNING: no JWT_SECRET set, generated random secret (sessions will not survive restarts)")
	}

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	// Run migrations automatically
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("failed to set goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("database migrations up to date")

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("failed to ping redis: %v", err)
	}

	queries := database.New(db)

	// Start session cleanup job
	session.StartCleanupJob(ctx, queries)

	router := server.NewRouter(queries, rdb, jwtSecret)

	log.Printf("server listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
