package test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/flenzero/aeon-backend/internal/chain"
)

func TestSolanaJSONRPCContractAgainstLocalServer(t *testing.T) {
	var mu sync.Mutex
	called := map[string]int{}
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var request struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode rpc request: %v", err)
		}
		mu.Lock()
		called[request.Method]++
		mu.Unlock()
		result := any(nil)
		switch request.Method {
		case "getSlot":
			result = uint64(42)
		case "getSignaturesForAddress":
			result = []map[string]any{{"signature": "sig-1", "slot": 41, "err": nil}}
		case "getTransaction":
			result = map[string]any{
				"slot": 41,
				"meta": map[string]any{
					"preTokenBalances": []any{
						map[string]any{"accountIndex": 0, "mint": "MintAEB", "owner": "player", "uiTokenAmount": map[string]any{"amount": "200", "decimals": 0}},
						map[string]any{"accountIndex": 1, "mint": "MintAEB", "owner": "treasury", "uiTokenAmount": map[string]any{"amount": "0", "decimals": 0}},
					},
					"postTokenBalances": []any{
						map[string]any{"accountIndex": 0, "mint": "MintAEB", "owner": "player", "uiTokenAmount": map[string]any{"amount": "150", "decimals": 0}},
						map[string]any{"accountIndex": 1, "mint": "MintAEB", "owner": "treasury", "uiTokenAmount": map[string]any{"amount": "50", "decimals": 0}},
					},
				},
			}
		case "getSignatureStatuses":
			result = map[string]any{"value": []any{map[string]any{"confirmationStatus": "finalized", "err": nil, "slot": 41}}}
		case "sendTransaction":
			result = "sent-signature"
		case "getLatestBlockhash":
			result = map[string]any{"value": map[string]any{"blockhash": "11111111111111111111111111111111"}}
		case "getAccountInfo":
			result = map[string]any{"value": map[string]any{"lamports": 1}}
		case "getTokenAccountBalance":
			result = map[string]any{"value": map[string]any{"amount": "1234"}}
		default:
			t.Fatalf("unexpected rpc method %q", request.Method)
		}
		raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(raw)),
			Request:    r,
		}, nil
	})

	client := chain.NewHTTPClient("http://solana-rpc.local")
	client.HTTP = &http.Client{Transport: transport}
	ctx := context.Background()
	if slot, err := client.GetSlot(ctx); err != nil || slot != 42 {
		t.Fatalf("slot=%d err=%v", slot, err)
	}
	if rows, err := client.GetSignaturesForAddress(ctx, "treasury", "", 10); err != nil || len(rows) != 1 || rows[0].Signature != "sig-1" {
		t.Fatalf("signatures=%+v err=%v", rows, err)
	}
	tx, err := client.GetTransaction(ctx, "sig-1")
	if err != nil {
		t.Fatal(err)
	}
	credits := chain.InboundTokenCredits(tx, "treasury", "MintAEB")
	if len(credits) != 1 || credits[0].AmountRaw != 50 || credits[0].FromOwner != "player" {
		t.Fatalf("credits=%+v", credits)
	}
	if statuses, err := client.GetSignatureStatuses(ctx, []string{"sig-1"}); err != nil || len(statuses) != 1 || statuses[0].ConfirmationStatus != "finalized" {
		t.Fatalf("statuses=%+v err=%v", statuses, err)
	}
	if sig, err := client.SendTransaction(ctx, "base64-transaction"); err != nil || sig != "sent-signature" {
		t.Fatalf("signature=%q err=%v", sig, err)
	}
	if hash, err := client.GetLatestBlockhash(ctx); err != nil || hash == "" {
		t.Fatalf("blockhash=%q err=%v", hash, err)
	}
	if exists, err := client.AccountExists(ctx, "account"); err != nil || !exists {
		t.Fatalf("exists=%v err=%v", exists, err)
	}
	if balance, err := client.GetTokenAccountBalanceRaw(ctx, "token-account"); err != nil || balance != 1234 {
		t.Fatalf("balance=%d err=%v", balance, err)
	}

	wantMethods := []string{"getSlot", "getSignaturesForAddress", "getTransaction", "getSignatureStatuses", "sendTransaction", "getLatestBlockhash", "getAccountInfo", "getTokenAccountBalance"}
	for _, method := range wantMethods {
		if called[method] != 1 {
			t.Fatalf("%s called %d times", method, called[method])
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
