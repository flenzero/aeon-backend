package httpx

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/security"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type contextKey string

const accountIDKey contextKey = "account_id"
const adminActorKey contextKey = "admin_actor"
const serviceIdentityKey contextKey = "service_identity"

type AdminActor struct {
	ID   string
	Role string
}

func WithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Super-Admin-Key, X-Internal-Key, X-Account-Id, X-Character-Id, X-Session-Id, X-Service-Id, X-Service-Timestamp, X-Service-Nonce, X-Service-Signature")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
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

type ServiceIdentityStore interface {
	ServiceIdentity(serviceID string) (store.ServiceIdentity, error)
	ConsumeServiceNonce(serviceID, nonce string, expiresAt time.Time) error
}

type AdminAuthStore interface {
	AdminUser(adminID string) (store.AdminUser, error)
}

func RequireService(cfg config.Config, identities ServiceIdentityStore, capability string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if (cfg.Profile == config.ProfileTest || cfg.Profile == config.ProfileDevelopment) && strings.TrimSpace(cfg.InternalKey) != "" && r.Header.Get("X-Internal-Key") == cfg.InternalKey {
			legacy := store.ServiceIdentity{ServiceID: "legacy-internal-key", Kind: "LEGACY", Status: store.ServiceIdentityActive, Capabilities: []string{capability}}
			ctx := context.WithValue(r.Context(), serviceIdentityKey, legacy)
			next(w, r.WithContext(ctx))
			return
		}
		serviceID := strings.ToLower(strings.TrimSpace(r.Header.Get(security.HeaderServiceID)))
		identity, err := identities.ServiceIdentity(serviceID)
		if err != nil || identity.Status != store.ServiceIdentityActive {
			Error(w, http.StatusUnauthorized, 4010, "service identity is invalid or disabled")
			return
		}
		expiresAt, err := security.VerifyServiceRequest(r, identity.PublicKey, time.Now().UTC(), cfg.ServiceAuthMaxSkew())
		if err != nil {
			Error(w, http.StatusUnauthorized, 4010, err.Error())
			return
		}
		if !identity.HasCapability(capability) {
			Error(w, http.StatusForbidden, 4030, "service identity lacks required capability")
			return
		}
		if err := identities.ConsumeServiceNonce(identity.ServiceID, r.Header.Get(security.HeaderServiceNonce), expiresAt); err != nil {
			Error(w, http.StatusUnauthorized, 4010, "service request nonce was already used")
			return
		}
		ctx := context.WithValue(r.Context(), serviceIdentityKey, identity)
		next(w, r.WithContext(ctx))
	}
}

func ContextServiceIdentity(r *http.Request) (store.ServiceIdentity, bool) {
	identity, ok := r.Context().Value(serviceIdentityKey).(store.ServiceIdentity)
	return identity, ok
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

func RequireAdmin(cfg config.Config, admins AdminAuthStore, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if superKey := strings.TrimSpace(cfg.SuperAdminOpsKey); superKey != "" && r.Header.Get("X-Super-Admin-Key") == superKey {
			ctx := context.WithValue(r.Context(), adminActorKey, AdminActor{ID: "super-admin-ops", Role: "SUPER_ADMIN"})
			next(w, r.WithContext(ctx))
			return
		}
		token := security.BearerToken(r.Header.Get("Authorization"))
		if token == "" {
			Error(w, http.StatusUnauthorized, 401, "admin token is invalid")
			return
		}
		if allowStaticAdminToken(cfg) && strings.TrimSpace(cfg.AdminToken) != "" && token == cfg.AdminToken {
			// Keep old local/test deployments working until they configure a distinct
			// super-admin key. Staging/production use signed admin-login JWTs.
			actor := AdminActor{ID: "admin-token", Role: "ADMIN"}
			if strings.TrimSpace(cfg.SuperAdminOpsKey) == "" {
				actor = AdminActor{ID: "bootstrap-super-admin", Role: "SUPER_ADMIN"}
			}
			ctx := context.WithValue(r.Context(), adminActorKey, actor)
			next(w, r.WithContext(ctx))
			return
		}
		if admins == nil || strings.TrimSpace(cfg.JWTSecret) == "" {
			Error(w, http.StatusUnauthorized, 401, "admin token is invalid")
			return
		}
		claims, err := security.VerifyAdminAccessToken(cfg.JWTSecret, token)
		if err != nil {
			Error(w, http.StatusUnauthorized, 401, "admin token is invalid or expired")
			return
		}
		admin, err := admins.AdminUser(claims.Subject)
		if err != nil || admin.Status != store.AdminUserActive {
			Error(w, http.StatusUnauthorized, 401, "admin user is invalid or disabled")
			return
		}
		role := strings.ToUpper(strings.TrimSpace(admin.Role))
		if role == "" {
			role = "ADMIN"
		}
		ctx := context.WithValue(r.Context(), adminActorKey, AdminActor{ID: admin.AdminID, Role: role})
		next(w, r.WithContext(ctx))
	}
}

func RequireSuperAdmin(cfg config.Config, admins AdminAuthStore, next http.HandlerFunc) http.HandlerFunc {
	return RequireAdmin(cfg, admins, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := ContextAdminActor(r)
		if !ok || actor.Role != "SUPER_ADMIN" {
			Error(w, http.StatusForbidden, 403, "super admin role is required")
			return
		}
		next(w, r)
	})
}

func allowStaticAdminToken(cfg config.Config) bool {
	return cfg.Profile == config.ProfileTest || cfg.Profile == config.ProfileDevelopment || cfg.Profile == ""
}

func ContextAdminActor(r *http.Request) (AdminActor, bool) {
	actor, ok := r.Context().Value(adminActorKey).(AdminActor)
	return actor, ok
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
