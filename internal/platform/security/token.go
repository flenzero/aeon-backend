package security

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/flenzero/aeon-backend/internal/chain"
)

type Claims struct {
	Subject   int64  `json:"sub"`
	Username  string `json:"username"`
	ExpiresAt int64  `json:"exp"`
}

type AdminClaims struct {
	Subject   string `json:"sub"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	ExpiresAt int64  `json:"exp"`
}

func RandomToken(prefix string) string {
	var buf [24]byte
	_, _ = rand.Read(buf[:])
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(buf[:])
}

func SignAccessToken(secret string, accountID int64, username string, ttl time.Duration) (string, error) {
	claims := Claims{
		Subject:   accountID,
		Username:  username,
		ExpiresAt: time.Now().Add(ttl).Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	sig := sign(secret, encoded)
	return encoded + "." + sig, nil
}

func SignAdminAccessToken(secret, adminID, username, role string, ttl time.Duration) (string, time.Time, error) {
	expiresAt := time.Now().Add(ttl)
	claims := AdminClaims{
		Subject:   strings.TrimSpace(adminID),
		Username:  strings.TrimSpace(username),
		Role:      strings.ToUpper(strings.TrimSpace(role)),
		ExpiresAt: expiresAt.Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	sig := sign(secret, encoded)
	return encoded + "." + sig, expiresAt, nil
}

func VerifyAccessToken(secret, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return Claims{}, errors.New("token is malformed")
	}
	expected := sign(secret, parts[0])
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return Claims{}, errors.New("token signature is invalid")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, err
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, err
	}
	if claims.Subject <= 0 || claims.ExpiresAt < time.Now().Unix() {
		return Claims{}, errors.New("token is expired")
	}
	return claims, nil
}

func VerifyAdminAccessToken(secret, token string) (AdminClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return AdminClaims{}, errors.New("token is malformed")
	}
	expected := sign(secret, parts[0])
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return AdminClaims{}, errors.New("token signature is invalid")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return AdminClaims{}, err
	}
	var claims AdminClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return AdminClaims{}, err
	}
	claims.Subject = strings.TrimSpace(claims.Subject)
	claims.Role = strings.ToUpper(strings.TrimSpace(claims.Role))
	if claims.Subject == "" || claims.ExpiresAt < time.Now().Unix() {
		return AdminClaims{}, errors.New("token is expired")
	}
	return claims, nil
}

func BearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}

func sign(secret, value string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TicketMessage(accountID, characterID int64, nonce string) string {
	return fmt.Sprintf("Aeonblight game ticket account=%d character=%d nonce=%s", accountID, characterID, nonce)
}

func AdminLoginMessage(adminID, nonce string) string {
	return "Sign in to Aeonblight Admin\nAdmin: " + strings.TrimSpace(adminID) + "\nNonce: " + strings.TrimSpace(nonce) + "\nThis signature authorizes a short-lived admin session only."
}

func VerifyEd25519Signature(publicKey, message, signatureText string) error {
	key, err := chain.DecodeBase58(strings.TrimSpace(publicKey))
	if err != nil || len(key) != ed25519.PublicKeySize {
		return errors.New("public key must be a 32-byte Ed25519 base58 public key")
	}
	signature, err := chain.DecodeSignature(strings.TrimSpace(signatureText))
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(key), []byte(message), signature) {
		return errors.New("signature is invalid")
	}
	return nil
}

func ParseID(value string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}
