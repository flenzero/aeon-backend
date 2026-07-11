package httpx

import (
	"context"
	"fmt"
	"net/http"

	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/security"
)

type contextKey string

const accountIDKey contextKey = "account_id"

func WithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Internal-Key, X-Account-Id, X-Character-Id, X-Session-Id")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if value := recover(); value != nil {
				Error(w, http.StatusInternalServerError, 500, fmt.Sprintf("internal server error: %v", value))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func RequireInternal(cfg config.Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Key") != cfg.InternalKey {
			Error(w, http.StatusUnauthorized, 4010, "internal key is invalid")
			return
		}
		next(w, r)
	}
}

func RequireJWT(cfg config.Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := security.BearerToken(r.Header.Get("Authorization"))
		if token == "" {
			Error(w, http.StatusUnauthorized, 401, "Authorization header is required")
			return
		}
		claims, err := security.VerifyAccessToken(cfg.JWTSecret, token)
		if err != nil {
			Error(w, http.StatusUnauthorized, 401, "Token is invalid or expired")
			return
		}
		ctx := context.WithValue(r.Context(), accountIDKey, claims.Subject)
		next(w, r.WithContext(ctx))
	}
}

func RequireAdmin(cfg config.Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if security.BearerToken(r.Header.Get("Authorization")) != cfg.AdminToken {
			Error(w, http.StatusUnauthorized, 401, "admin token is invalid")
			return
		}
		next(w, r)
	}
}

func ContextAccountID(r *http.Request) (int64, bool) {
	id, ok := r.Context().Value(accountIDKey).(int64)
	return id, ok
}

func Health(service string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		OK(w, map[string]any{"service": service, "status": "ok"})
	}
}
