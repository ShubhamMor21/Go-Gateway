package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/ShubhamMor21/go-gateway/internal/constants"
)

// Config holds every runtime parameter consumed by the gateway.
// All values originate from environment variables; constants are used as fallbacks only.
type Config struct {
	// Server
	ServerPort              string
	MetricsPort             string
	LogLevel                string
	ReadTimeout             time.Duration
	WriteTimeout            time.Duration
	IdleTimeout             time.Duration
	MaxConcurrency          int
	MaxRequestSize          int // bytes
	GracefulShutdownTimeout time.Duration

	// Auth
	JWTSecret string

	// Redis
	RedisURL string

	// Kafka
	KafkaBrokers        []string
	KafkaRetries        int
	KafkaRetryBackoffMs int
	KafkaTLSEnabled     bool
	KafkaSASLEnabled    bool
	KafkaSASLUsername   string
	KafkaSASLPassword   string
	KafkaSASLMechanism  string // PLAIN | SCRAM-SHA-256 | SCRAM-SHA-512

	// gRPC
	GRPCServiceURL         string
	GRPCMaxRetries         int
	GRPCRetryBaseDelay     time.Duration
	GRPCTLSEnabled         bool
	GRPCServerNameOverride string // for self-signed certs in staging

	// Rate limiting
	RateLimitRequests      int
	RateLimitWindowSeconds int
	RateLimitFailOpen      bool // false = fail-closed (safer for fintech)

	// Cache
	CacheTTL time.Duration

	// Circuit breaker
	CBMaxRequests  uint32
	CBInterval     time.Duration
	CBTimeout      time.Duration
	CBFailureRatio float64

	// Load shedding
	LoadShedMaxConnections int

	// CORS — required to be set explicitly in production; default "*" is dev-only
	CORSAllowedOrigins string

	// Request timeout
	RequestTimeout time.Duration

	// Metrics
	MetricsAuthToken string // if set, require Bearer token on /metrics
}

// Load reads .env (if present) then environment variables and returns a validated Config.
// Missing required secrets cause an immediate fatal-level error — the gateway must not start
// without them.
func Load() (*Config, error) {
	// .env is optional; in production, variables are injected by the orchestrator.
	_ = godotenv.Load()

	cfg := &Config{
		// Server
		ServerPort:              envString("SERVER_PORT", constants.DefaultServerPort),
		MetricsPort:             envString("METRICS_PORT", constants.DefaultMetricsPort),
		LogLevel:                envString("LOG_LEVEL", constants.DefaultLogLevel),
		ReadTimeout:             envDuration("FIBER_READ_TIMEOUT_SECONDS", constants.DefaultFiberReadTimeoutSeconds),
		WriteTimeout:            envDuration("FIBER_WRITE_TIMEOUT_SECONDS", constants.DefaultFiberWriteTimeoutSeconds),
		IdleTimeout:             envDuration("FIBER_IDLE_TIMEOUT_SECONDS", constants.DefaultFiberIdleTimeoutSeconds),
		MaxConcurrency:          envInt("FIBER_CONCURRENCY", constants.DefaultFiberConcurrency),
		MaxRequestSize:          envInt("MAX_REQUEST_SIZE_MB", constants.DefaultMaxRequestSizeMB) * 1024 * 1024,
		GracefulShutdownTimeout: envDuration("GRACEFUL_SHUTDOWN_TIMEOUT_SECONDS", constants.DefaultGracefulShutdownSeconds),

		// Auth — required
		JWTSecret: envString("JWT_SECRET", ""),

		// Redis — required
		RedisURL: envString("REDIS_URL", ""),

		// Kafka
		KafkaBrokers:        envStringSlice("KAFKA_BROKERS", ","),
		KafkaRetries:        envInt("KAFKA_RETRIES", constants.DefaultKafkaRetries),
		KafkaRetryBackoffMs: envInt("KAFKA_RETRY_BACKOFF_MS", constants.DefaultKafkaRetryBackoffMs),
		KafkaTLSEnabled:     envBool("KAFKA_TLS_ENABLED", constants.DefaultKafkaTLSEnabled),
		KafkaSASLEnabled:    envBool("KAFKA_SASL_ENABLED", false),
		KafkaSASLUsername:   envString("KAFKA_SASL_USERNAME", ""),
		KafkaSASLPassword:   envString("KAFKA_SASL_PASSWORD", ""),
		KafkaSASLMechanism:  envString("KAFKA_SASL_MECHANISM", constants.DefaultKafkaSASLMechanism),

		// gRPC
		GRPCServiceURL:         envString("GRPC_SERVICE_URL", ""),
		GRPCMaxRetries:         envInt("GRPC_MAX_RETRIES", constants.DefaultGRPCMaxRetries),
		GRPCRetryBaseDelay:     envDurationMs("GRPC_RETRY_BASE_DELAY_MS", constants.DefaultGRPCRetryBaseDelayMs),
		GRPCTLSEnabled:         envBool("GRPC_TLS_ENABLED", constants.DefaultGRPCTLSEnabled),
		GRPCServerNameOverride: envString("GRPC_SERVER_NAME_OVERRIDE", ""),

		// Rate limiting
		RateLimitRequests:      envInt("RATE_LIMIT_REQUESTS", constants.DefaultRateLimitRequests),
		RateLimitWindowSeconds: envInt("RATE_LIMIT_WINDOW_SECONDS", constants.DefaultRateLimitWindowSeconds),
		RateLimitFailOpen:      envBool("RATE_LIMIT_FAIL_OPEN", constants.DefaultRateLimitFailOpen),

		// Cache
		CacheTTL: envDuration("CACHE_TTL_SECONDS", constants.DefaultCacheTTLSeconds),

		// Circuit breaker
		CBMaxRequests:  uint32(envInt("CB_MAX_REQUESTS", int(constants.DefaultCBMaxRequests))),
		CBInterval:     envDuration("CB_INTERVAL_SECONDS", constants.DefaultCBIntervalSeconds),
		CBTimeout:      envDuration("CB_TIMEOUT_SECONDS", constants.DefaultCBTimeoutSeconds),
		CBFailureRatio: envFloat64("CB_FAILURE_RATIO", constants.DefaultCBFailureRatio),

		// Load shedding
		LoadShedMaxConnections: envInt("LOAD_SHED_MAX_CONNECTIONS", constants.DefaultLoadShedMaxConnections),

		// CORS
		CORSAllowedOrigins: envString("CORS_ALLOWED_ORIGINS", "*"),

		// Request timeout
		RequestTimeout: envDuration("REQUEST_TIMEOUT_SECONDS", constants.DefaultRequestTimeoutSeconds),

		// Metrics
		MetricsAuthToken: envString("METRICS_AUTH_TOKEN", ""),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if len(c.JWTSecret) < constants.JWTSecretMinLength {
		return fmt.Errorf("JWT_SECRET must be at least %d bytes for HS256 security", constants.JWTSecretMinLength)
	}
	if c.RedisURL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}
	if !isValidPort(c.ServerPort) {
		return fmt.Errorf("SERVER_PORT %q is not a valid port number", c.ServerPort)
	}
	if !isValidPort(c.MetricsPort) {
		return fmt.Errorf("METRICS_PORT %q is not a valid port number", c.MetricsPort)
	}
	if c.RateLimitRequests <= 0 {
		return fmt.Errorf("RATE_LIMIT_REQUESTS must be > 0")
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("REQUEST_TIMEOUT_SECONDS must be > 0")
	}
	if c.KafkaSASLEnabled && (c.KafkaSASLUsername == "" || c.KafkaSASLPassword == "") {
		return fmt.Errorf("KAFKA_SASL_USERNAME and KAFKA_SASL_PASSWORD are required when KAFKA_SASL_ENABLED=true")
	}
	return nil
}

// ──────────────────────────────────────────────
// helpers
// ──────────────────────────────────────────────

func envString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func envFloat64(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func envDuration(key string, fallbackSeconds int) time.Duration {
	secs := envInt(key, fallbackSeconds)
	return time.Duration(secs) * time.Second
}

func envDurationMs(key string, fallbackMs int) time.Duration {
	ms := envInt(key, fallbackMs)
	return time.Duration(ms) * time.Millisecond
}

func envStringSlice(key, sep string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func isValidPort(port string) bool {
	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535
}
