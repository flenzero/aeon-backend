package security

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flenzero/aeon-backend/internal/chain"
)

const MaxSignedServiceBodyBytes int64 = 1 << 20

const (
	HeaderServiceID        = "X-Service-Id"
	HeaderServiceTimestamp = "X-Service-Timestamp"
	HeaderServiceNonce     = "X-Service-Nonce"
	HeaderServiceSignature = "X-Service-Signature"
)

func SignServiceRequest(req *http.Request, serviceID string, privateKey ed25519.PrivateKey, now time.Time, nonce string) error {
	if req == nil {
		return errors.New("request is required")
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return errors.New("private key must be a 64-byte Ed25519 private key")
	}
	serviceID = strings.ToLower(strings.TrimSpace(serviceID))
	nonce = strings.TrimSpace(nonce)
	if serviceID == "" || !validServiceNonce(nonce) {
		return errors.New("serviceId and a 16-128 character nonce are required")
	}
	timestamp := strconv.FormatInt(now.UTC().Unix(), 10)
	body, err := signedRequestBody(req)
	if err != nil {
		return err
	}
	message := serviceRequestMessage(serviceID, timestamp, nonce, req.Method, requestTarget(req), body)
	signature := ed25519.Sign(privateKey, []byte(message))
	req.Header.Set(HeaderServiceID, serviceID)
	req.Header.Set(HeaderServiceTimestamp, timestamp)
	req.Header.Set(HeaderServiceNonce, nonce)
	req.Header.Set(HeaderServiceSignature, chain.EncodeBase58(signature))
	return nil
}

func ParseEd25519PrivateKey(value string) (ed25519.PrivateKey, error) {
	decoded, err := chain.DecodeBase58(strings.TrimSpace(value))
	if err != nil || len(decoded) != ed25519.PrivateKeySize {
		return nil, errors.New("private key must be a 64-byte Ed25519 base58 private key")
	}
	return ed25519.PrivateKey(decoded), nil
}

func VerifyServiceRequest(req *http.Request, publicKey string, now time.Time, maxSkew time.Duration) (time.Time, error) {
	if req == nil {
		return time.Time{}, errors.New("request is required")
	}
	if maxSkew <= 0 {
		maxSkew = 2 * time.Minute
	}
	serviceID := strings.ToLower(strings.TrimSpace(req.Header.Get(HeaderServiceID)))
	timestamp := strings.TrimSpace(req.Header.Get(HeaderServiceTimestamp))
	nonce := strings.TrimSpace(req.Header.Get(HeaderServiceNonce))
	signatureText := strings.TrimSpace(req.Header.Get(HeaderServiceSignature))
	if serviceID == "" || timestamp == "" || !validServiceNonce(nonce) || signatureText == "" {
		return time.Time{}, errors.New("service signature headers are incomplete")
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return time.Time{}, errors.New("service timestamp is invalid")
	}
	signedAt := time.Unix(seconds, 0).UTC()
	if signedAt.Before(now.Add(-maxSkew)) || signedAt.After(now.Add(maxSkew)) {
		return time.Time{}, errors.New("service signature timestamp is outside the allowed window")
	}
	key, err := chain.DecodeBase58(strings.TrimSpace(publicKey))
	if err != nil || len(key) != ed25519.PublicKeySize {
		return time.Time{}, errors.New("service public key is invalid")
	}
	signature, err := chain.DecodeSignature(signatureText)
	if err != nil {
		return time.Time{}, err
	}
	body, err := signedRequestBody(req)
	if err != nil {
		return time.Time{}, err
	}
	message := serviceRequestMessage(serviceID, timestamp, nonce, req.Method, requestTarget(req), body)
	if !ed25519.Verify(ed25519.PublicKey(key), []byte(message), signature) {
		return time.Time{}, errors.New("service signature is invalid")
	}
	return now.Add(maxSkew), nil
}

func signedRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return []byte{}, nil
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, MaxSignedServiceBodyBytes+1))
	if err != nil {
		return nil, err
	}
	req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(body))
	if int64(len(body)) > MaxSignedServiceBodyBytes {
		return nil, fmt.Errorf("request body exceeds %d bytes", MaxSignedServiceBodyBytes)
	}
	return body, nil
}

func requestTarget(req *http.Request) string {
	target := req.URL.EscapedPath()
	if req.URL.RawQuery != "" {
		target += "?" + req.URL.RawQuery
	}
	return target
}

func serviceRequestMessage(serviceID, timestamp, nonce, method, target string, body []byte) string {
	hash := sha256.Sum256(body)
	return strings.Join([]string{
		"AEONBLIGHT-SERVICE-V1",
		serviceID,
		timestamp,
		nonce,
		strings.ToUpper(strings.TrimSpace(method)),
		target,
		hex.EncodeToString(hash[:]),
	}, "\n")
}

func validServiceNonce(nonce string) bool {
	if len(nonce) < 16 || len(nonce) > 128 {
		return false
	}
	for _, r := range nonce {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("._:-", r) {
			continue
		}
		return false
	}
	return true
}
