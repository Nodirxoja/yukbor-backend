package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/aventiseld/yukbor-backend/internal/orders"
	"github.com/aventiseld/yukbor-backend/pkg/config"
)

func main() {
	cfg := config.Load("8082")
	h := orders.NewHandler(cfg)
	slog.Info("orders service listening", "port", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, h.Routes()); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
