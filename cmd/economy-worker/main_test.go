package main

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/flenzero/aeon-backend/internal/chain"
	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/security"
)

func TestProductionWorkerSignsInternalRequest(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		status := http.StatusOK
		if _, err := security.VerifyServiceRequest(r, chain.EncodeBase58(publicKey), time.Now().UTC(), 2*time.Minute); err != nil {
			status = http.StatusUnauthorized
		}
		return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d %s", status, http.StatusText(status)), Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}, nil
	})}

	cfg := config.Config{
		Profile: config.ProfileProduction, EconomyAPIURL: "https://economy.internal",
		ServiceID: "economy-worker-primary", ServicePrivateKey: chain.EncodeBase58(privateKey),
	}
	result := postInternal(context.Background(), client, cfg, "/api/economy/internal/unlocks/settle")
	if !strings.HasPrefix(result, "200 OK") {
		t.Fatalf("result = %q", result)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
