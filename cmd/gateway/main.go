package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/aventiseld/yukbor-backend/internal/gateway"
	"github.com/aventiseld/yukbor-backend/pkg/config"
)

func main() {
	cfg := config.Load("8080")
	slog.Info("gateway listening", "port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, gateway.Routes(cfg)); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
