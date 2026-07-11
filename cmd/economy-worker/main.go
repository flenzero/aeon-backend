package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/flenzero/aeon-backend/internal/platform/config"
)

func main() {
	cfg := config.Load("economy-worker", "")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := &http.Client{Timeout: 10 * time.Second}
	log.Printf("%s started interval=%s economy=%s", cfg.ServiceName, cfg.WorkerInterval, cfg.EconomyAPIURL)

	ticker := time.NewTicker(cfg.WorkerInterval)
	defer ticker.Stop()
	runOnce(ctx, client, cfg)
	for {
		select {
		case <-ctx.Done():
			log.Printf("%s stopped", cfg.ServiceName)
			return
		case <-ticker.C:
			runOnce(ctx, client, cfg)
		}
	}
}

func runOnce(ctx context.Context, client *http.Client, cfg config.Config) {
	start := time.Now()
	results := map[string]string{}
	results["settleUnlocks"] = postInternal(ctx, client, cfg, "/api/economy/internal/unlocks/settle")
	results["processWithdrawals"] = postInternal(ctx, client, cfg, "/api/economy/internal/withdrawals/process")
	results["scanDeposits"] = postInternal(ctx, client, cfg, "/api/economy/internal/chain/deposits/scan")
	results["submitPayouts"] = postInternal(ctx, client, cfg, "/api/economy/internal/chain/payouts/submit")
	results["confirmPayouts"] = postInternal(ctx, client, cfg, "/api/economy/internal/chain/payouts/confirm")
	log.Printf("worker tick completed in %s: %+v", time.Since(start).Round(time.Millisecond), results)
}

func postInternal(ctx context.Context, client *http.Client, cfg config.Config, path string) string {
	url := strings.TrimRight(cfg.EconomyAPIURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", cfg.InternalKey)
	resp, err := client.Do(req)
	if err != nil {
		return err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return resp.Status + " " + strings.TrimSpace(string(body))
}
