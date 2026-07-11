package store

import (
	"context"
	"log"

	"github.com/flenzero/aeon-backend/internal/platform/config"
)

func Open(ctx context.Context, cfg config.Config) (Repository, func()) {
	if cfg.DatabaseURL == "" {
		log.Printf("%s using in-memory store; set DATABASE_URL for shared Postgres persistence", cfg.ServiceName)
		return Default, func() {}
	}
	pg, err := NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open postgres store: %v", err)
	}
	log.Printf("%s using Postgres store", cfg.ServiceName)
	return pg, pg.Close
}
