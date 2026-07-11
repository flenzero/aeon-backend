package chain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RPC is the Solana JSON-RPC surface used by deposit scan and payout confirm.
type RPC interface {
	GetSlot(ctx context.Context) (uint64, error)
	GetSignaturesForAddress(ctx context.Context, address string, before string, limit int) ([]SignatureInfo, error)
	GetTransaction(ctx context.Context, signature string) (*TransactionDetail, error)
	GetSignatureStatuses(ctx context.Context, signatures []string) ([]SignatureStatus, error)
	SendTransaction(ctx context.Context, base64Tx string) (string, error)
	GetLatestBlockhash(ctx context.Context) (string, error)
	AccountExists(ctx context.Context, address string) (bool, error)
	GetTokenAccountBalanceRaw(ctx context.Context, tokenAccount string) (uint64, error)
}

type SignatureInfo struct {
	Signature string
	Slot      uint64
	Err       any
}

type SignatureStatus struct {
	Signature          string
	Confirmations      *int
	ConfirmationStatus string
	Err                any
	Slot               uint64
}

type TokenBalance struct {
	AccountIndex  int
	Mint          string
	Owner         string
	Amount        uint64
	Decimals      int
	UITokenAmount string
}

type TransactionDetail struct {
	Signature         string
	Slot              uint64
	PreTokenBalances  []TokenBalance
	PostTokenBalances []TokenBalance
}

type TokenCredit struct {
	Signature      string
	Slot           uint64
	Mint           string
	FromOwner      string
	ToOwner        string
	ToTokenAccount string
	AmountRaw      uint64
}

type HTTPClient struct {
	URL        string
	HTTP       *http.Client
	Commitment string
}

func NewHTTPClient(rpcURL string) *HTTPClient {
	return &HTTPClient{
		URL:        strings.TrimSpace(rpcURL),
		HTTP:       &http.Client{Timeout: 20 * time.Second},
		Commitment: "confirmed",
	}
}

func (c *HTTPClient) call(ctx context.Context, method string, params []any, result any) error {
	if strings.TrimSpace(c.URL) == "" {
		return errors.New("solana rpc url is empty")
	}
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	var envelope struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return fmt.Errorf("solana rpc %s: %s", method, envelope.Error.Message)
	}
	if result == nil {
		return nil
	}
	return json.Unmarshal(envelope.Result, result)
}

func (c *HTTPClient) GetSlot(ctx context.Context) (uint64, error) {
	var slot uint64
	err := c.call(ctx, "getSlot", []any{map[string]any{"commitment": c.Commitment}}, &slot)
	return slot, err
}

func (c *HTTPClient) GetSignaturesForAddress(ctx context.Context, address string, before string, limit int) ([]SignatureInfo, error) {
	if limit <= 0 {
		limit = 50
	}
	opts := map[string]any{"limit": limit, "commitment": c.Commitment}
	if strings.TrimSpace(before) != "" {
		opts["before"] = before
	}
	var rows []struct {
		Signature string `json:"signature"`
		Slot      uint64 `json:"slot"`
		Err       any    `json:"err"`
	}
	if err := c.call(ctx, "getSignaturesForAddress", []any{address, opts}, &rows); err != nil {
		return nil, err
	}
	out := make([]SignatureInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, SignatureInfo{Signature: row.Signature, Slot: row.Slot, Err: row.Err})
	}
	return out, nil
}

func (c *HTTPClient) GetTransaction(ctx context.Context, signature string) (*TransactionDetail, error) {
	var raw map[string]any
	err := c.call(ctx, "getTransaction", []any{
		signature,
		map[string]any{
			"encoding":                       "jsonParsed",
			"commitment":                     c.Commitment,
			"maxSupportedTransactionVersion": 0,
		},
	}, &raw)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	return parseTransactionDetail(signature, raw)
}

func (c *HTTPClient) GetSignatureStatuses(ctx context.Context, signatures []string) ([]SignatureStatus, error) {
	if len(signatures) == 0 {
		return nil, nil
	}
	var result struct {
		Value []struct {
			Confirmations      *int   `json:"confirmations"`
			ConfirmationStatus string `json:"confirmationStatus"`
			Err                any    `json:"err"`
			Slot               uint64 `json:"slot"`
		} `json:"value"`
	}
	if err := c.call(ctx, "getSignatureStatuses", []any{signatures, map[string]any{"searchTransactionHistory": true}}, &result); err != nil {
		return nil, err
	}
	out := make([]SignatureStatus, len(signatures))
	for i, sig := range signatures {
		out[i] = SignatureStatus{Signature: sig}
		if i < len(result.Value) {
			out[i].Confirmations = result.Value[i].Confirmations
			out[i].ConfirmationStatus = result.Value[i].ConfirmationStatus
			out[i].Err = result.Value[i].Err
			out[i].Slot = result.Value[i].Slot
		}
	}
	return out, nil
}

func (c *HTTPClient) SendTransaction(ctx context.Context, base64Tx string) (string, error) {
	var sig string
	err := c.call(ctx, "sendTransaction", []any{
		base64Tx,
		map[string]any{"encoding": "base64", "skipPreflight": false, "preflightCommitment": c.Commitment},
	}, &sig)
	return sig, err
}

func (c *HTTPClient) GetLatestBlockhash(ctx context.Context) (string, error) {
	var result struct {
		Value struct {
			Blockhash string `json:"blockhash"`
		} `json:"value"`
	}
	if err := c.call(ctx, "getLatestBlockhash", []any{map[string]any{"commitment": c.Commitment}}, &result); err != nil {
		return "", err
	}
	if result.Value.Blockhash == "" {
		return "", errors.New("empty blockhash")
	}
	return result.Value.Blockhash, nil
}

func (c *HTTPClient) AccountExists(ctx context.Context, address string) (bool, error) {
	var result any
	if err := c.call(ctx, "getAccountInfo", []any{address, map[string]any{"encoding": "base64", "commitment": c.Commitment}}, &result); err != nil {
		return false, err
	}
	raw, _ := json.Marshal(result)
	var parsed struct {
		Value any `json:"value"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return false, err
	}
	return parsed.Value != nil, nil
}

func (c *HTTPClient) GetTokenAccountBalanceRaw(ctx context.Context, tokenAccount string) (uint64, error) {
	var result struct {
		Value struct {
			Amount string `json:"amount"`
		} `json:"value"`
	}
	if err := c.call(ctx, "getTokenAccountBalance", []any{tokenAccount, map[string]any{"commitment": c.Commitment}}, &result); err != nil {
		return 0, err
	}
	return strconv.ParseUint(result.Value.Amount, 10, 64)
}

func parseTransactionDetail(signature string, raw map[string]any) (*TransactionDetail, error) {
	detail := &TransactionDetail{Signature: signature}
	if slot, ok := asUint64(raw["slot"]); ok {
		detail.Slot = slot
	}
	meta, _ := raw["meta"].(map[string]any)
	if meta == nil {
		return detail, nil
	}
	detail.PreTokenBalances = parseTokenBalances(meta["preTokenBalances"])
	detail.PostTokenBalances = parseTokenBalances(meta["postTokenBalances"])
	return detail, nil
}

func parseTokenBalances(raw any) []TokenBalance {
	rows, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]TokenBalance, 0, len(rows))
	for _, item := range rows {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		ui, _ := row["uiTokenAmount"].(map[string]any)
		amountStr, _ := ui["amount"].(string)
		amount, _ := strconv.ParseUint(amountStr, 10, 64)
		decimals, _ := asInt(ui["decimals"])
		accountIndex, _ := asInt(row["accountIndex"])
		out = append(out, TokenBalance{
			AccountIndex:  accountIndex,
			Mint:          stringField(row["mint"]),
			Owner:         stringField(row["owner"]),
			Amount:        amount,
			Decimals:      decimals,
			UITokenAmount: amountStr,
		})
	}
	return out
}

// InboundTokenCredits returns positive balance deltas for the configured deposit owner/mint.
func InboundTokenCredits(tx *TransactionDetail, depositOwner, mint string) []TokenCredit {
	if tx == nil {
		return nil
	}
	depositOwner = strings.TrimSpace(depositOwner)
	mint = strings.TrimSpace(mint)
	pre := map[string]uint64{}
	for _, row := range tx.PreTokenBalances {
		if mint != "" && row.Mint != mint {
			continue
		}
		pre[row.Owner+"|"+row.Mint] = row.Amount
	}
	var credits []TokenCredit
	for _, row := range tx.PostTokenBalances {
		if mint != "" && row.Mint != mint {
			continue
		}
		if depositOwner != "" && row.Owner != depositOwner {
			continue
		}
		before := pre[row.Owner+"|"+row.Mint]
		if row.Amount <= before {
			continue
		}
		delta := row.Amount - before
		fromOwner := findDecreasedOwner(tx, mint, row.Owner, delta)
		credits = append(credits, TokenCredit{
			Signature: tx.Signature,
			Slot:      tx.Slot,
			Mint:      row.Mint,
			FromOwner: fromOwner,
			ToOwner:   row.Owner,
			AmountRaw: delta,
		})
	}
	return credits
}

func findDecreasedOwner(tx *TransactionDetail, mint, excludeOwner string, delta uint64) string {
	pre := map[string]uint64{}
	for _, row := range tx.PreTokenBalances {
		if mint != "" && row.Mint != mint {
			continue
		}
		pre[row.Owner] = row.Amount
	}
	for _, row := range tx.PostTokenBalances {
		if mint != "" && row.Mint != mint {
			continue
		}
		if row.Owner == excludeOwner {
			continue
		}
		before := pre[row.Owner]
		if before >= row.Amount && before-row.Amount == delta {
			return row.Owner
		}
	}
	for owner, before := range pre {
		if owner == excludeOwner {
			continue
		}
		found := false
		for _, row := range tx.PostTokenBalances {
			if row.Owner == owner && (mint == "" || row.Mint == mint) {
				found = true
				if before > row.Amount {
					return owner
				}
			}
		}
		if !found && before > 0 {
			return owner
		}
	}
	return ""
}

func RawToGameAmount(raw uint64, decimals int) int64 {
	if decimals <= 0 {
		return int64(raw)
	}
	div := uint64(1)
	for i := 0; i < decimals; i++ {
		div *= 10
	}
	return int64(raw / div)
}

func stringField(v any) string {
	s, _ := v.(string)
	return s
}

func asUint64(v any) (uint64, bool) {
	switch n := v.(type) {
	case float64:
		return uint64(n), true
	case json.Number:
		u, err := strconv.ParseUint(string(n), 10, 64)
		return u, err == nil
	case string:
		u, err := strconv.ParseUint(n, 10, 64)
		return u, err == nil
	default:
		return 0, false
	}
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case json.Number:
		i, err := strconv.Atoi(string(n))
		return i, err == nil
	default:
		return 0, false
	}
}

// MemoryRPC is a test double.
type MemoryRPC struct {
	Slot          uint64
	Signatures    map[string][]SignatureInfo // address -> sigs
	Txs           map[string]*TransactionDetail
	Statuses      map[string]SignatureStatus
	Sent          []string
	Accounts      map[string]bool
	TokenBalances map[string]uint64
}

func (m *MemoryRPC) GetSlot(ctx context.Context) (uint64, error) {
	if m.Slot == 0 {
		return 1, nil
	}
	return m.Slot, nil
}

func (m *MemoryRPC) GetSignaturesForAddress(ctx context.Context, address string, before string, limit int) ([]SignatureInfo, error) {
	rows := append([]SignatureInfo{}, m.Signatures[address]...)
	if before != "" {
		filtered := make([]SignatureInfo, 0, len(rows))
		skip := true
		for _, row := range rows {
			if row.Signature == before {
				skip = false
				continue
			}
			if !skip {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (m *MemoryRPC) GetTransaction(ctx context.Context, signature string) (*TransactionDetail, error) {
	return m.Txs[signature], nil
}

func (m *MemoryRPC) GetSignatureStatuses(ctx context.Context, signatures []string) ([]SignatureStatus, error) {
	out := make([]SignatureStatus, 0, len(signatures))
	for _, sig := range signatures {
		if st, ok := m.Statuses[sig]; ok {
			out = append(out, st)
			continue
		}
		out = append(out, SignatureStatus{Signature: sig, ConfirmationStatus: "finalized"})
	}
	return out, nil
}

func (m *MemoryRPC) SendTransaction(ctx context.Context, base64Tx string) (string, error) {
	sig := "stub_sent_" + strconv.Itoa(len(m.Sent)+1)
	m.Sent = append(m.Sent, base64Tx)
	return sig, nil
}

func (m *MemoryRPC) GetLatestBlockhash(ctx context.Context) (string, error) {
	return "11111111111111111111111111111111", nil
}

func (m *MemoryRPC) AccountExists(ctx context.Context, address string) (bool, error) {
	if m.Accounts == nil {
		return true, nil
	}
	return m.Accounts[address], nil
}

func (m *MemoryRPC) GetTokenAccountBalanceRaw(ctx context.Context, tokenAccount string) (uint64, error) {
	if m.TokenBalances == nil {
		return 0, nil
	}
	return m.TokenBalances[tokenAccount], nil
}
