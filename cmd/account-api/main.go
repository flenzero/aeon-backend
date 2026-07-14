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
	if err := cfg.ValidateStartup(); err != nil {
		log.Fatal(err)
	}
	st, closeStore := store.Open(context.Background(), cfg)
	defer closeStore()
	handler, err := account.OpenHandler(cfg, st)
	if err != nil {
		log.Fatalf("open account runtime dependencies: %v", err)
	}
	defer handler.Close()
	log.Printf("%s listening on %s", cfg.ServiceName, cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, handler.Routes()))
}
