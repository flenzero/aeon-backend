package main

import (
	"context"
	"log"
	"net/http"

	"github.com/flenzero/aeon-backend/internal/admin"
	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func main() {
	cfg := config.Load("admin-api", ":8083")
	st, closeStore := store.Open(context.Background(), cfg)
	defer closeStore()
	handler := admin.NewHandler(cfg, st)
	log.Printf("%s listening on %s", cfg.ServiceName, cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, handler.Routes()))
}
