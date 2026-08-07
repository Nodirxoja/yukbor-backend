package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/aventiseld/yukbor-backend/internal/auth"
	"github.com/aventiseld/yukbor-backend/pkg/config"
	"github.com/aventiseld/yukbor-backend/pkg/db"
)

func main() {
	cfg := config.Load("8081")

	pool, err := db.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("database unreachable", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	h := auth.NewHandler(cfg, pool)
	slog.Info("auth service listening", "port", cfg.Port, "env", cfg.AppEnv)
	if err := http.ListenAndServe(":"+cfg.Port, h.Routes()); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
