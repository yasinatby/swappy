package database

import (
	"context"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Reset drops everything in the public schema, re-applies all migrations and
// loads the seed. Local development only.
func Reset(ctx context.Context, pool *pgxpool.Pool, migrations fs.FS, seedSQL string) error {
	if _, err := pool.Exec(ctx, `drop schema public cascade; create schema public;`); err != nil {
		return err
	}
	if err := Migrate(ctx, pool, migrations); err != nil {
		return err
	}
	_, err := pool.Exec(ctx, seedSQL)
	return err
}
