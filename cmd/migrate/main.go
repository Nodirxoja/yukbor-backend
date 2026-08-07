// Command migrate applies the embedded SQL migrations and exits. It runs as a
// one-shot compose service that every other service depends on, so `make up`
// always starts against an up-to-date schema.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/aventiseld/yukbor-backend/migrations"
	"github.com/aventiseld/yukbor-backend/pkg/config"
	"github.com/aventiseld/yukbor-backend/pkg/db"
)

func main() {
	cfg := config.Load("0")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}
	slog.Info("migrations up to date")
}
