package readiness

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/flenzero/aeon-backend/internal/platform/config"
)

type Database interface {
	Ping(context.Context) error
	HasSchemaVersion(context.Context, string) (bool, error)
}

func PersistenceChecks(cfg config.Config, dependency any) []Check {
	database, isDatabase := dependency.(Database)
	if !isDatabase {
		err := errors.New("in-memory persistence adapter is active")
		if cfg.RequiresDatabase() {
			return []Check{Required("postgres", func(context.Context) error { return err })}
		}
		return []Check{Optional("postgres", func(context.Context) error { return err })}
	}

	checks := []Check{
		Required("postgres", database.Ping),
	}
	version := strings.TrimSpace(cfg.RequiredSchemaVersion)
	checks = append(checks, Required("schema", func(ctx context.Context) error {
		if version == "" {
			return errors.New("REQUIRED_SCHEMA_VERSION is empty")
		}
		exists, err := database.HasSchemaVersion(ctx, version)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("required schema version %s is not applied; run the independent migration command", version)
		}
		return nil
	}))
	return checks
}
