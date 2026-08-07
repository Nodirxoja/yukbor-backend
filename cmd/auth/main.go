package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/aventiseld/yukbor-backend/internal/auth"
	"github.com/aventiseld/yukbor-backend/pkg/config"
)

func main() {
	cfg := config.Load("8081")
	h := auth.NewHandler(cfg)
	slog.Info("auth service listening", "port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, h.Routes()); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
