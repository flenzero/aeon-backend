package security

import (
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
)

type Claims struct {
	Subject   int64  `json:"sub"`
	Username  string `json:"username"`
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

func ParseID(value string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}
