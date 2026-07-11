package main

import (
	"context"
	"log"
	"net/http"

	"github.com/flenzero/aeon-backend/internal/account"
	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func main() {
	cfg := config.Load("account-api", ":8081")
	st, closeStore := store.Open(context.Background(), cfg)
	defer closeStore()
	handler := account.NewHandler(cfg, st)
	log.Printf("%s listening on %s", cfg.ServiceName, cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, handler.Routes()))
}
