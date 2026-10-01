// Package testdb gives integration tests a freshly migrated, throwaway
// database on the local Postgres from compose.yaml.
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yasinatby/swappy/backend/internal/database"
	"github.com/yasinatby/swappy/backend/migrations"
)

const defaultURL = "postgres://swappy:swappy@localhost:5433/swappy?sslmode=disable"

// New creates a database, applies all migrations and drops it after the test.
// Set SWAPPY_TEST_DATABASE_URL to point at another server.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	adminURL := os.Getenv("SWAPPY_TEST_DATABASE_URL")
	if adminURL == "" {
		adminURL = defaultURL
	}
	admin, err := database.Open(ctx, adminURL)
	if err != nil {
		t.Skipf("no test database (run `docker compose up -d db`): %v", err)
	}
	defer admin.Close()

	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix)
	name := "swappy_test_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, "create database "+name); err != nil {
		t.Fatalf("create test database: %v", err)
	}

	parsed, _ := url.Parse(adminURL)
	parsed.Path = "/" + name
	pool, err := database.Open(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		admin, err := database.Open(context.Background(), adminURL)
		if err != nil {
			return
		}
		defer admin.Close()
		_, _ = admin.Exec(context.Background(), "drop database if exists "+name+" with (force)")
	})

	if err := database.Migrate(ctx, pool, migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}
