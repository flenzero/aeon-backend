package security

import (
	"crypto/ed25519"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/flenzero/aeon-backend/internal/chain"
)

func TestServiceRequestSignatureBoundaries(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	newSigned := func(t *testing.T, signedAt time.Time, nonce, body string) *http.Request {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/internal/job?limit=20", strings.NewReader(body))
		if err := SignServiceRequest(req, "worker-primary", privateKey, signedAt, nonce); err != nil {
			t.Fatal(err)
		}
		return req
	}

	t.Run("valid edge of window", func(t *testing.T) {
		req := newSigned(t, now.Add(-119*time.Second), "nonce-valid-window-01", `{}`)
		if _, err := VerifyServiceRequest(req, chain.EncodeBase58(publicKey), now, 2*time.Minute); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("expired timestamp", func(t *testing.T) {
		req := newSigned(t, now.Add(-121*time.Second), "nonce-expired-time-01", `{}`)
		if _, err := VerifyServiceRequest(req, chain.EncodeBase58(publicKey), now, 2*time.Minute); err == nil {
			t.Fatal("expired signature accepted")
		}
	})
	t.Run("future timestamp", func(t *testing.T) {
		req := newSigned(t, now.Add(121*time.Second), "nonce-future-time-001", `{}`)
		if _, err := VerifyServiceRequest(req, chain.EncodeBase58(publicKey), now, 2*time.Minute); err == nil {
			t.Fatal("future signature accepted")
		}
	})
	t.Run("tampered body", func(t *testing.T) {
		req := newSigned(t, now, "nonce-tampered-body1", `{"amount":1}`)
		req.Body = io.NopCloser(strings.NewReader(`{"amount":2}`))
		if _, err := VerifyServiceRequest(req, chain.EncodeBase58(publicKey), now, 2*time.Minute); err == nil {
			t.Fatal("tampered body accepted")
		}
	})
	t.Run("tampered target", func(t *testing.T) {
		req := newSigned(t, now, "nonce-tampered-path1", `{}`)
		req.URL.RawQuery = "limit=21"
		if _, err := VerifyServiceRequest(req, chain.EncodeBase58(publicKey), now, 2*time.Minute); err == nil {
			t.Fatal("tampered target accepted")
		}
	})
	t.Run("short nonce", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/internal/job", strings.NewReader(`{}`))
		if err := SignServiceRequest(req, "worker-primary", privateKey, now, "short"); err == nil {
			t.Fatal("short nonce accepted")
		}
	})
	t.Run("oversized body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/internal/job", strings.NewReader(strings.Repeat("x", int(MaxSignedServiceBodyBytes)+1)))
		if err := SignServiceRequest(req, "worker-primary", privateKey, now, "nonce-oversized-body1"); err == nil {
			t.Fatal("oversized signed body accepted")
		}
	})
}
