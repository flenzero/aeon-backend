package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	action := "status"
	if len(os.Args) > 1 {
		action = strings.TrimSpace(os.Args[1])
	}
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		log.Fatal("db migration refused: DATABASE_URL is required")
	}
	migrationsDir := strings.TrimSpace(os.Getenv("MIGRATIONS_DIR"))
	if migrationsDir == "" {
		migrationsDir = "/app/migrations"
	}
	if _, err := os.Stat(migrationsDir); err != nil {
		migrationsDir = "migrations"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("ping postgres: %v", err)
	}

	switch action {
	case "bootstrap":
		if err := bootstrap(ctx, pool, migrationsDir); err != nil {
			log.Fatal(err)
		}
	case "up":
		if err := up(ctx, pool, migrationsDir); err != nil {
			log.Fatal(err)
		}
	case "status":
		if err := status(ctx, pool); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("usage: db-migrate {bootstrap|up|status}")
	}
}

func bootstrap(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	hasTable, err := schemaTableExists(ctx, pool)
	if err != nil {
		return err
	}
	if hasTable {
		return fmt.Errorf("db migration refused: bootstrap is only for a new database; run 'db-migrate up' for an existing database")
	}
	sql, err := os.ReadFile(filepath.Join(migrationsDir, "aeonblight_full_schema.sql"))
	if err != nil {
		return err
	}
	fmt.Println("Applying canonical Aeonblight schema")
	return execSQL(ctx, pool, string(sql))
}

func up(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	hasTable, err := schemaTableExists(ctx, pool)
	if err != nil {
		return err
	}
	if !hasTable {
		return fmt.Errorf("db migration refused: schema_migrations is missing; run 'db-migrate bootstrap' explicitly for a new database")
	}
	files, err := filepath.Glob(filepath.Join(migrationsDir, "updates", "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		version := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		applied, err := versionApplied(ctx, pool, version)
		if err != nil {
			return err
		}
		if applied {
			fmt.Printf("SKIP %s\n", version)
			continue
		}
		sql, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		fmt.Printf("APPLY %s\n", version)
		if err := execSQL(ctx, pool, string(sql)); err != nil {
			return err
		}
	}
	return nil
}

func status(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `
		SELECT version, applied_at
		FROM schema_migrations
		ORDER BY applied_at, version
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var version string
		var appliedAt time.Time
		if err := rows.Scan(&version, &appliedAt); err != nil {
			return err
		}
		fmt.Printf("%s\t%s\n", version, appliedAt.Format(time.RFC3339))
	}
	return rows.Err()
}

func schemaTableExists(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, "SELECT to_regclass('public.schema_migrations') IS NOT NULL").Scan(&exists)
	return exists, err
}

func versionApplied(ctx context.Context, pool *pgxpool.Pool, version string) (bool, error) {
	var applied bool
	err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&applied)
	return applied, err
}

func execSQL(ctx context.Context, pool *pgxpool.Pool, sql string) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	results := conn.Conn().PgConn().Exec(ctx, sql)
	for results.NextResult() {
		if _, err := results.ResultReader().Close(); err != nil {
			return err
		}
	}
	if err := results.Close(); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return fmt.Errorf("%s: %s", pgErr.Code, pgErr.Message)
		}
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}
	return nil
}
