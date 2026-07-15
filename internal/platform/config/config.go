package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServiceName           string
	Profile               Profile
	TestScope             TestScope
	StubMode              StubMode
	AllowRedisFallback    bool
	Addr                  string
	EconomyAPIURL         string
	EconomyConfigDir      string
	GameClientBaseURL     string
	TokenSymbol           string
	SupportWallets        []string
	DatabaseURL           string
	RequiredSchemaVersion string
	InternalKey           string
	JWTSecret             string
	AdminToken            string
	SuperAdminOpsKey      string
	AdminSessionTTLMin    int
	ServiceID             string
	ServicePrivateKey     string
	ServiceAuthMaxSkewSec int
	WorkerInterval        time.Duration
	AutoWithdrawSingleMax int64
	UserDailyWithdrawMax  int64
	GlobalHourlyMax       int64
	GlobalDailyMax        int64

	// Solana chain (Phase 4). Disabled by default; worker/API no-op until configured.
	SolanaRPCEnabled          bool
	SolanaRPCURL              string
	SolanaNetwork             string
	SolanaTokenMint           string
	SolanaTokenDecimals       int
	SolanaDepositWallet       string
	SolanaPayoutWallet        string
	SolanaPayoutPrivateKey    string
	SolanaPayoutMode          string // stub | record | rpc
	SolanaDepositScanLimit    int
	SolanaPayoutLowBalanceRaw uint64

	// Redis session / online coordination (account-api).
	RedisEnabled         bool
	RedisAddr            string
	RedisPassword        string
	RedisDB              int
	SessionTTLHours      int
	OnlinePresenceTTLSec int

	loadIssues []string
}

func Load(serviceName, defaultAddr string) Config {
	loadLocalEnvFile()
	profile := Profile(strings.ToLower(strings.TrimSpace(env("APP_PROFILE", env("AEONBLIGHT_ENV", string(ProfileDevelopment))))))
	defaultStub := StubEnabled
	if profile == ProfileStaging || profile == ProfileProduction {
		defaultStub = StubDisabled
	}
	defaultAdminToken := "dev-admin-token"
	if profile == ProfileStaging || profile == ProfileProduction {
		defaultAdminToken = ""
	}
	allowRedisFallback, allowRedisFallbackIssue := envBoolValidated("ALLOW_REDIS_FALLBACK", false)
	cfg := Config{
		ServiceName:               serviceName,
		Profile:                   profile,
		TestScope:                 TestScope(strings.ToLower(strings.TrimSpace(env("TEST_SCOPE", string(TestScopeContract))))),
		StubMode:                  StubMode(strings.ToLower(strings.TrimSpace(env("STUB_MODE", string(defaultStub))))),
		AllowRedisFallback:        allowRedisFallback,
		Addr:                      env("ADDR", defaultAddr),
		EconomyAPIURL:             env("ECONOMY_API_URL", "http://localhost:8082"),
		EconomyConfigDir:          env("ECONOMY_CONFIG_DIR", "configs/economy"),
		GameClientBaseURL:         env("GAME_CLIENT_BASE_URL", ""),
		TokenSymbol:               env("TOKEN_SYMBOL", "AEB"),
		SupportWallets:            envList("SUPPORT_WALLETS", []string{"phantom", "solflare", "backpack", "okx"}),
		DatabaseURL:               env("DATABASE_URL", ""),
		RequiredSchemaVersion:     env("REQUIRED_SCHEMA_VERSION", "20260715_account_launch_admission_v1"),
		InternalKey:               envAllowEmpty("INTERNAL_KEY", "dev-internal-key"),
		JWTSecret:                 env("JWT_SECRET", "dev-jwt-secret"),
		AdminToken:                env("ADMIN_TOKEN", defaultAdminToken),
		SuperAdminOpsKey:          env("SUPER_ADMIN_OPS_KEY", ""),
		AdminSessionTTLMin:        envInt("ADMIN_SESSION_TTL_MINUTES", 30),
		ServiceID:                 env("SERVICE_ID", ""),
		ServicePrivateKey:         env("SERVICE_PRIVATE_KEY", ""),
		ServiceAuthMaxSkewSec:     envInt("SERVICE_AUTH_MAX_SKEW_SECONDS", 120),
		WorkerInterval:            time.Duration(envInt("WORKER_INTERVAL_SECONDS", 15)) * time.Second,
		AutoWithdrawSingleMax:     int64(envInt("AUTO_WITHDRAW_SINGLE_MAX", 5000)),
		UserDailyWithdrawMax:      int64(envInt("USER_DAILY_WITHDRAW_MAX", 20000)),
		GlobalHourlyMax:           int64(envInt("GLOBAL_HOURLY_WITHDRAW_MAX", 30000)),
		GlobalDailyMax:            int64(envInt("GLOBAL_DAILY_WITHDRAW_MAX", 150000)),
		SolanaRPCEnabled:          envBool("SOLANA_RPC_ENABLED", false),
		SolanaRPCURL:              env("SOLANA_RPC_URL", "https://api.devnet.solana.com"),
		SolanaNetwork:             env("SOLANA_NETWORK", "solana-devnet"),
		SolanaTokenMint:           env("SOLANA_TOKEN_MINT", ""),
		SolanaTokenDecimals:       envInt("SOLANA_TOKEN_DECIMALS", 9),
		SolanaDepositWallet:       env("SOLANA_DEPOSIT_WALLET", ""),
		SolanaPayoutWallet:        env("SOLANA_PAYOUT_WALLET", ""),
		SolanaPayoutPrivateKey:    env("SOLANA_PAYOUT_PRIVATE_KEY", ""),
		SolanaPayoutMode:          env("SOLANA_PAYOUT_MODE", "stub"),
		SolanaDepositScanLimit:    envInt("SOLANA_DEPOSIT_SCAN_LIMIT", 50),
		SolanaPayoutLowBalanceRaw: uint64(envInt("SOLANA_PAYOUT_LOW_BALANCE_RAW", 0)),
		RedisEnabled:              envBool("REDIS_ENABLED", true),
		RedisAddr:                 env("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:             env("REDIS_PASSWORD", ""),
		RedisDB:                   envInt("REDIS_DB", 0),
		SessionTTLHours:           envInt("SESSION_TTL_HOURS", 168),
		OnlinePresenceTTLSec:      envInt("ONLINE_PRESENCE_TTL_SECONDS", 90),
	}
	if allowRedisFallbackIssue != "" {
		cfg.loadIssues = append(cfg.loadIssues, allowRedisFallbackIssue)
	}
	return cfg
}

func (c Config) ServiceAuthMaxSkew() time.Duration {
	seconds := c.ServiceAuthMaxSkewSec
	if seconds <= 0 {
		seconds = 120
	}
	return time.Duration(seconds) * time.Second
}

func (c Config) AdminSessionTTL() time.Duration {
	minutes := c.AdminSessionTTLMin
	if minutes <= 0 {
		minutes = 30
	}
	return time.Duration(minutes) * time.Minute
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envAllowEmpty(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envList(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return append([]string(nil), fallback...)
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return append([]string(nil), fallback...)
	}
	return out
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envBoolValidated(key string, fallback bool) (bool, string) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, ""
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true, ""
	case "0", "false", "no", "off":
		return false, ""
	default:
		return fallback, key + " must be a boolean (true/false)"
	}
}
