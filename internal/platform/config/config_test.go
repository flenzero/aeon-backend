package config

import (
	"crypto/ed25519"
	"strings"
	"testing"
	"time"

	"github.com/flenzero/aeon-backend/internal/chain"
)

func TestProductionWorkerRequiresEd25519ServiceIdentity(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		ServiceName: "economy-worker", Profile: ProfileProduction, TestScope: TestScopeFull,
		StubMode: StubDisabled, EconomyAPIURL: "https://economy.internal", WorkerInterval: 15 * time.Second,
	}
	err = cfg.ValidateStartup()
	if err == nil {
		t.Fatal("ValidateStartup() accepted worker without service identity")
	}
	for _, want := range []string{"SERVICE_ID", "SERVICE_PRIVATE_KEY"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ValidateStartup() error = %q, want %q", err, want)
		}
	}

	cfg.ServiceID = "economy-worker-primary"
	cfg.ServicePrivateKey = chain.EncodeBase58(privateKey)
	if err := cfg.ValidateStartup(); err != nil {
		t.Fatalf("configured production worker rejected: %v", err)
	}
}

func TestValidateStartupReportsEveryMissingRequirement(t *testing.T) {
	cfg := Config{
		ServiceName: "account-api",
		Profile:     ProfileDevelopment,
		StubMode:    StubEnabled,
	}

	err := cfg.ValidateStartup()
	if err == nil {
		t.Fatal("ValidateStartup() error = nil, want aggregated startup rejection")
	}

	for _, requirement := range []string{"DATABASE_URL", "REDIS_ENABLED", "INTERNAL_KEY", "JWT_SECRET"} {
		if !strings.Contains(err.Error(), requirement) {
			t.Errorf("ValidateStartup() error = %q, want %s", err, requirement)
		}
	}
}

func TestProductionRefusesEveryStubSwitch(t *testing.T) {
	cfg := Config{
		ServiceName:         "economy-api",
		Profile:             ProfileProduction,
		StubMode:            StubEnabled,
		DatabaseURL:         "postgres://configured",
		InternalKey:         "configured",
		JWTSecret:           "configured",
		SolanaRPCEnabled:    true,
		SolanaRPCURL:        "https://rpc.example",
		SolanaTokenMint:     "mint",
		SolanaDepositWallet: "deposit",
		SolanaPayoutWallet:  "payout",
		SolanaPayoutMode:    "stub",
	}

	err := cfg.ValidateStartup()
	if err == nil {
		t.Fatal("ValidateStartup() error = nil, want production stub rejection")
	}
	for _, want := range []string{"STUB_MODE=disabled", "SOLANA_PAYOUT_MODE cannot be stub"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ValidateStartup() error = %q, want %q", err, want)
		}
	}
}

func TestLoadReadsRuntimeProfileControls(t *testing.T) {
	t.Setenv("AEONBLIGHT_SKIP_LOCAL_ENV", "true")
	t.Setenv("APP_PROFILE", "test")
	t.Setenv("TEST_SCOPE", "contract")
	t.Setenv("STUB_MODE", "enabled")
	t.Setenv("ALLOW_REDIS_FALLBACK", "true")

	cfg := Load("account-api", ":8081")
	if cfg.Profile != ProfileTest || cfg.TestScope != TestScopeContract || cfg.StubMode != StubEnabled || !cfg.AllowRedisFallback {
		t.Fatalf("runtime controls = profile:%q scope:%q stub:%q redisFallback:%v", cfg.Profile, cfg.TestScope, cfg.StubMode, cfg.AllowRedisFallback)
	}
}

func TestLoadReadsPublicHomeConfig(t *testing.T) {
	t.Setenv("AEONBLIGHT_SKIP_LOCAL_ENV", "true")
	t.Setenv("APP_PROFILE", "test")
	t.Setenv("TOKEN_SYMBOL", "AEBT")
	t.Setenv("SUPPORT_WALLETS", "phantom, solflare,,okx")

	cfg := Load("account-api", ":8081")
	if cfg.TokenSymbol != "AEBT" {
		t.Fatalf("TokenSymbol = %q", cfg.TokenSymbol)
	}
	want := []string{"phantom", "solflare", "okx"}
	if len(cfg.SupportWallets) != len(want) {
		t.Fatalf("SupportWallets = %#v", cfg.SupportWallets)
	}
	for i := range want {
		if cfg.SupportWallets[i] != want[i] {
			t.Fatalf("SupportWallets = %#v", cfg.SupportWallets)
		}
	}
}

func TestLoadAllowsExplicitEmptyInternalKey(t *testing.T) {
	t.Setenv("AEONBLIGHT_SKIP_LOCAL_ENV", "true")
	t.Setenv("APP_PROFILE", "staging")
	t.Setenv("INTERNAL_KEY", "")

	cfg := Load("account-api", ":8081")
	if cfg.InternalKey != "" {
		t.Fatalf("InternalKey = %q, want explicit empty value", cfg.InternalKey)
	}
}

func TestTestScopeControlsDependencyRequirements(t *testing.T) {
	base := Config{
		ServiceName: "account-api",
		Profile:     ProfileTest,
		StubMode:    StubEnabled,
		InternalKey: "configured",
		JWTSecret:   "configured",
	}
	base.TestScope = TestScopeContract
	if err := base.ValidateStartup(); err != nil {
		t.Fatalf("contract scope rejected memory adapters: %v", err)
	}

	base.TestScope = TestScopeFull
	err := base.ValidateStartup()
	if err == nil {
		t.Fatal("full scope accepted missing persistence dependencies")
	}
	for _, want := range []string{"DATABASE_URL", "REDIS_ENABLED"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ValidateStartup() error = %q, want %s", err, want)
		}
	}
}

func TestDisabledStubRequiresCompleteSolanaConfiguration(t *testing.T) {
	cfg := Config{
		ServiceName: "economy-api",
		Profile:     ProfileDevelopment,
		StubMode:    StubDisabled,
		DatabaseURL: "postgres://configured",
		InternalKey: "configured",
		JWTSecret:   "configured",
	}

	err := cfg.ValidateStartup()
	if err == nil {
		t.Fatal("ValidateStartup() error = nil, want incomplete Solana rejection")
	}
	for _, want := range []string{
		"SOLANA_RPC_ENABLED", "SOLANA_RPC_URL", "SOLANA_NETWORK", "SOLANA_TOKEN_MINT",
		"SOLANA_DEPOSIT_WALLET", "SOLANA_PAYOUT_WALLET", "SOLANA_PAYOUT_MODE",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ValidateStartup() error = %q, want %s", err, want)
		}
	}
}

func TestLoadRejectsInvalidRuntimeControlsWithoutSilentFallback(t *testing.T) {
	t.Setenv("AEONBLIGHT_SKIP_LOCAL_ENV", "true")
	t.Setenv("APP_PROFILE", "preview")
	t.Setenv("TEST_SCOPE", "everything")
	t.Setenv("STUB_MODE", "sometimes")
	t.Setenv("ALLOW_REDIS_FALLBACK", "perhaps")

	err := Load("economy-worker", "").ValidateStartup()
	if err == nil {
		t.Fatal("ValidateStartup() error = nil, want invalid runtime control rejection")
	}
	for _, want := range []string{"APP_PROFILE", "TEST_SCOPE", "STUB_MODE", "ALLOW_REDIS_FALLBACK"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ValidateStartup() error = %q, want %s", err, want)
		}
	}
}

func TestProductionRejectsDevelopmentSecrets(t *testing.T) {
	cfg := Config{
		ServiceName:           "account-api",
		Profile:               ProfileProduction,
		TestScope:             TestScopeFull,
		StubMode:              StubDisabled,
		DatabaseURL:           "postgres://configured",
		RequiredSchemaVersion: "schema-v1",
		InternalKey:           "dev-internal-key",
		JWTSecret:             "dev-jwt-secret",
		SolanaPayoutMode:      "rpc",
	}
	err := cfg.ValidateStartup()
	if err == nil || !strings.Contains(err.Error(), "INTERNAL_KEY") || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("ValidateStartup() error = %v, want both development-secret rejections", err)
	}
}
