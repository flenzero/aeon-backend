package main

import (
	"context"
	"log"
	"net/http"

	"github.com/flenzero/aeon-backend/internal/economy"
	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func main() {
	cfg := config.Load("economy-api", ":8082")
	st, closeStore := store.Open(context.Background(), cfg)
	defer closeStore()
	handler := economy.NewHandler(cfg, st)
	log.Printf("%s listening on %s", cfg.ServiceName, cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, handler.Routes()))
}
