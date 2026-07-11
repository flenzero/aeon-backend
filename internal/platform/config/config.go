package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ServiceName           string
	Addr                  string
	EconomyAPIURL         string
	EconomyConfigDir      string
	DatabaseURL           string
	InternalKey           string
	JWTSecret             string
	AdminToken            string
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
}

func Load(serviceName, defaultAddr string) Config {
	loadLocalEnvFile()
	return Config{
		ServiceName:               serviceName,
		Addr:                      env("ADDR", defaultAddr),
		EconomyAPIURL:             env("ECONOMY_API_URL", "http://localhost:8082"),
		EconomyConfigDir:          env("ECONOMY_CONFIG_DIR", "configs/economy"),
		DatabaseURL:               env("DATABASE_URL", ""),
		InternalKey:               env("INTERNAL_KEY", "dev-internal-key"),
		JWTSecret:                 env("JWT_SECRET", "dev-jwt-secret"),
		AdminToken:                env("ADMIN_TOKEN", "dev-admin-token"),
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
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
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
