package config

import (
	"crypto/ed25519"
	"fmt"
	"strings"

	"github.com/flenzero/aeon-backend/internal/chain"
)

type Profile string

const (
	ProfileTest        Profile = "test"
	ProfileDevelopment Profile = "development"
	ProfileStaging     Profile = "staging"
	ProfileProduction  Profile = "production"
)

type TestScope string

const (
	TestScopeUnit        TestScope = "unit"
	TestScopeContract    TestScope = "contract"
	TestScopeIntegration TestScope = "integration"
	TestScopeFull        TestScope = "full"
)

type StubMode string

const (
	StubEnabled  StubMode = "enabled"
	StubDisabled StubMode = "disabled"
)

type ValidationError struct {
	Service string
	Issues  []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s refused to start; configuration errors:\n- %s", e.Service, strings.Join(e.Issues, "\n- "))
}

func (c Config) ValidateStartup() error {
	issues := append(make([]string, 0, len(c.loadIssues)+8), c.loadIssues...)
	require := func(ok bool, message string) {
		if !ok {
			issues = append(issues, message)
		}
	}

	needsDatabase := c.RequiresDatabase()
	if !oneOfProfile(c.Profile) {
		issues = append(issues, "APP_PROFILE must be one of test, development, staging, production")
	}
	if !oneOfTestScope(c.TestScope) {
		issues = append(issues, "TEST_SCOPE must be one of unit, contract, integration, full")
	}
	if c.StubMode != StubEnabled && c.StubMode != StubDisabled {
		issues = append(issues, "STUB_MODE must be enabled or disabled")
	}
	if c.Profile == ProfileDevelopment {
		if needsDatabase {
			require(strings.TrimSpace(c.DatabaseURL) != "", "DATABASE_URL is required in development")
		}
		if c.ServiceName == "account-api" && !c.AllowRedisFallback {
			require(c.RedisEnabled, "REDIS_ENABLED=true is required; set ALLOW_REDIS_FALLBACK=true only for an explicit development fallback")
		}
	}
	if c.Profile == ProfileTest && (c.TestScope == TestScopeIntegration || c.TestScope == TestScopeFull) {
		if needsDatabase {
			require(strings.TrimSpace(c.DatabaseURL) != "", "DATABASE_URL is required for test integration/full scope")
		}
		if c.ServiceName == "account-api" {
			require(c.RedisEnabled, "REDIS_ENABLED=true is required for account-api test integration/full scope")
		}
	}
	if c.Profile == ProfileStaging || c.Profile == ProfileProduction {
		require(c.StubMode == StubDisabled, "STUB_MODE=disabled is required in staging and production")
		if needsDatabase {
			require(strings.TrimSpace(c.DatabaseURL) != "", "DATABASE_URL is required in staging and production")
		}
		if c.ServiceName == "account-api" {
			require(c.RedisEnabled, "REDIS_ENABLED=true is required for account-api in staging and production")
		}
		if c.ServiceName == "economy-api" {
			require(!strings.EqualFold(strings.TrimSpace(c.SolanaPayoutMode), "stub"), "SOLANA_PAYOUT_MODE cannot be stub in staging or production")
			require(strings.EqualFold(strings.TrimSpace(c.SolanaPayoutMode), "rpc"), "SOLANA_PAYOUT_MODE=rpc is required for economy-api in staging and production")
			require(strings.TrimSpace(c.SolanaPayoutPrivateKey) != "", "SOLANA_PAYOUT_PRIVATE_KEY is required for rpc payout mode")
		}
	}
	if c.ServiceName == "economy-api" && c.StubMode == StubDisabled {
		require(c.SolanaRPCEnabled, "SOLANA_RPC_ENABLED=true is required when STUB_MODE=disabled")
		require(strings.TrimSpace(c.SolanaRPCURL) != "", "SOLANA_RPC_URL is required when STUB_MODE=disabled")
		require(strings.TrimSpace(c.SolanaNetwork) != "", "SOLANA_NETWORK is required when STUB_MODE=disabled")
		require(strings.TrimSpace(c.SolanaTokenMint) != "", "SOLANA_TOKEN_MINT is required when STUB_MODE=disabled")
		require(strings.TrimSpace(c.SolanaDepositWallet) != "", "SOLANA_DEPOSIT_WALLET is required when STUB_MODE=disabled")
		require(strings.TrimSpace(c.SolanaPayoutWallet) != "", "SOLANA_PAYOUT_WALLET is required when STUB_MODE=disabled")
		require(strings.TrimSpace(c.SolanaPayoutMode) != "" && !strings.EqualFold(strings.TrimSpace(c.SolanaPayoutMode), "stub"), "SOLANA_PAYOUT_MODE must be record or rpc when STUB_MODE=disabled")
	}
	if c.ServiceName == "account-api" {
		require(strings.TrimSpace(c.JWTSecret) != "", "JWT_SECRET is required")
	}
	usesLegacyInternalKey := c.Profile == ProfileTest || c.Profile == ProfileDevelopment
	if usesLegacyInternalKey && (c.ServiceName == "account-api" || c.ServiceName == "economy-api" || c.ServiceName == "economy-worker") && strings.TrimSpace(c.ServiceID) == "" {
		require(strings.TrimSpace(c.InternalKey) != "", "INTERNAL_KEY is required when development/test legacy service authentication is used")
	}
	if c.ServiceName == "economy-worker" {
		require(strings.TrimSpace(c.EconomyAPIURL) != "", "ECONOMY_API_URL is required")
		require(c.WorkerInterval > 0, "WORKER_INTERVAL_SECONDS must be positive")
		if c.Profile == ProfileStaging || c.Profile == ProfileProduction {
			require(strings.TrimSpace(c.ServiceID) != "", "SERVICE_ID is required for economy-worker in staging and production")
			require(validEd25519PrivateKey(c.ServicePrivateKey), "SERVICE_PRIVATE_KEY must be a 64-byte Ed25519 base58 private key")
		}
	}
	if c.ServiceName == "admin-api" {
		require(strings.TrimSpace(c.JWTSecret) != "", "JWT_SECRET is required for admin signed-login tokens")
		require(c.AdminSessionTTLMin > 0, "ADMIN_SESSION_TTL_MINUTES must be positive")
		if (c.Profile == ProfileTest || c.Profile == ProfileDevelopment) && strings.TrimSpace(c.SuperAdminOpsKey) == "" {
			require(strings.TrimSpace(c.AdminToken) != "", "ADMIN_TOKEN is required when development/test bootstrap super-admin compatibility is used")
		}
		if c.Profile == ProfileStaging || c.Profile == ProfileProduction {
			require(strings.TrimSpace(c.SuperAdminOpsKey) != "", "SUPER_ADMIN_OPS_KEY is required for super-admin operations")
		}
	}
	if needsDatabase {
		require(strings.TrimSpace(c.RequiredSchemaVersion) != "", "REQUIRED_SCHEMA_VERSION is required")
	}
	if c.Profile == ProfileStaging || c.Profile == ProfileProduction {
		require(strings.TrimSpace(c.InternalKey) == "", "INTERNAL_KEY must be empty in staging and production; use an Ed25519 service identity")
		if c.ServiceName == "account-api" {
			require(!isDevelopmentSecret(c.JWTSecret), "JWT_SECRET must not use the development default in staging or production")
		}
		if c.ServiceName == "admin-api" {
			require(!isDevelopmentSecret(c.SuperAdminOpsKey), "SUPER_ADMIN_OPS_KEY must not use a development secret in staging or production")
		}
	}

	if len(issues) > 0 {
		return &ValidationError{Service: c.ServiceName, Issues: issues}
	}
	return nil
}

func validEd25519PrivateKey(value string) bool {
	decoded, err := chain.DecodeBase58(strings.TrimSpace(value))
	return err == nil && len(decoded) == ed25519.PrivateKeySize
}

func (c Config) RequiresDatabase() bool {
	if c.ServiceName == "economy-worker" {
		return false
	}
	if c.Profile == ProfileTest && (c.TestScope == TestScopeUnit || c.TestScope == TestScopeContract) {
		return false
	}
	return true
}

func oneOfProfile(profile Profile) bool {
	return profile == ProfileTest || profile == ProfileDevelopment || profile == ProfileStaging || profile == ProfileProduction
}

func oneOfTestScope(scope TestScope) bool {
	return scope == TestScopeUnit || scope == TestScopeContract || scope == TestScopeIntegration || scope == TestScopeFull
}

func isDevelopmentSecret(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "" || strings.HasPrefix(value, "dev-") || strings.Contains(value, "change-me")
}
