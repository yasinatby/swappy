package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yasinatby/swappy/backend/internal/auth"
	"github.com/yasinatby/swappy/backend/internal/config"
	"github.com/yasinatby/swappy/backend/internal/database"
	"github.com/yasinatby/swappy/backend/internal/items"
	"github.com/yasinatby/swappy/backend/internal/server"
	"github.com/yasinatby/swappy/backend/internal/storage"
	"github.com/yasinatby/swappy/backend/internal/swaps"
	"github.com/yasinatby/swappy/backend/internal/users"
	"github.com/yasinatby/swappy/backend/migrations"
	"github.com/yasinatby/swappy/backend/seed"
)

const usage = `usage: swappy-backend [serve|migrate|seed|reset]

  serve    apply pending migrations and start the HTTP server (default)
  migrate  apply pending migrations
  seed     load local development data
  reset    drop the schema, migrate and seed (local development only)`

func main() {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if err := run(context.Background(), command); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, command string) error {
	appConfig := config.Load()

	pool, err := database.Open(ctx, appConfig.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch command {
	case "serve":
		if err := database.Migrate(ctx, pool, migrations.FS); err != nil {
			return err
		}
		go runEvery(ctx, time.Minute, "expire_offers", func(ctx context.Context) error {
			n, err := swaps.NewService(pool).ExpireOffers(ctx)
			if n > 0 {
				log.Printf("expire_offers: expired %d offers", n)
			}
			return err
		})
		deps, err := buildDeps(appConfig, pool)
		if err != nil {
			return err
		}
		httpServer := server.New(appConfig, deps)
		log.Printf("swappy backend listening on %s", appConfig.Address())
		return httpServer.ListenAndServe()
	case "migrate":
		return database.Migrate(ctx, pool, migrations.FS)
	case "seed":
		_, err := pool.Exec(ctx, seed.SQL)
		return err
	case "reset":
		return database.Reset(ctx, pool, migrations.FS, seed.SQL)
	default:
		return fmt.Errorf("unknown command %q\n%s", command, usage)
	}
}

// runEvery runs job immediately and then on every tick until ctx is done.
func runEvery(ctx context.Context, interval time.Duration, name string, job func(context.Context) error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := job(ctx); err != nil {
			log.Printf("%s: %v", name, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func buildDeps(appConfig config.Config, pool *pgxpool.Pool) (server.Deps, error) {
	secret := appConfig.TokenSecret
	if secret == "" {
		if !appConfig.DevLogin {
			return server.Deps{}, errors.New("SWAPPY_TOKEN_SECRET must be set")
		}
		secret = "insecure-dev-secret"
	}
	store, err := storage.NewLocal(appConfig.StorageDir)
	if err != nil {
		return server.Deps{}, err
	}
	swapService := swaps.NewService(pool)
	return server.Deps{
		Tokens:  auth.NewTokens(secret, 30*24*time.Hour),
		Items:   items.NewService(pool),
		Swaps:   swapService,
		Users:   users.NewService(pool, swapService),
		Storage: store,
	}, nil
}
