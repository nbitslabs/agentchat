package session

import (
	"context"
	"log"
	"time"

	"github.com/nbitslabs/agentchat/internal/database"
)

// StartCleanupJob runs an hourly goroutine that deletes expired sessions.
func StartCleanupJob(ctx context.Context, queries *database.Queries) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				count, err := queries.DeleteExpiredSessions(ctx)
				if err != nil {
					log.Printf("session cleanup error: %v", err)
				} else if count > 0 {
					log.Printf("session cleanup: deleted %d expired sessions", count)
				}
			}
		}
	}()
}
