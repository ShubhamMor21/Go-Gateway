package constants

// Default/fallback values used ONLY when the corresponding ENV variable is absent.
// These are compile-time constants — no secrets, no environment-specific values.
const (
	DefaultServerPort                  = "3030"
	DefaultMetricsPort                 = "9090"
	DefaultLogLevel                    = "info"
	DefaultRateLimitRequests           = 100
	DefaultRateLimitWindowSeconds      = 60
	DefaultCacheTTLSeconds             = 300
	DefaultRequestTimeoutSeconds       = 30
	DefaultMaxRequestSizeMB            = 10
	DefaultGracefulShutdownSeconds     = 30
	DefaultCBMaxRequests               = 5   // half-open probe requests
	DefaultCBIntervalSeconds           = 60  // counts-reset interval in closed state
	DefaultCBTimeoutSeconds            = 30  // open → half-open wait
	DefaultCBFailureRatio              = 0.6 // 60 % errors trips the breaker
	DefaultLoadShedMaxConnections      = 10_000
	DefaultGRPCMaxRetries              = 3
	DefaultGRPCRetryBaseDelayMs        = 100
	DefaultFiberReadTimeoutSeconds     = 30
	DefaultFiberWriteTimeoutSeconds    = 30
	DefaultFiberIdleTimeoutSeconds     = 120
	DefaultFiberConcurrency            = 262_144 // 256 K goroutines ceiling
	DefaultKafkaRetries                = 3
	DefaultKafkaRetryBackoffMs         = 200
	DefaultKafkaSASLMechanism          = "PLAIN"
	DefaultGRPCTLSEnabled              = false
	DefaultKafkaTLSEnabled             = false
	DefaultRateLimitFailOpen           = false // fintech default: fail-closed
	JWTSecretMinLength                 = 32    // bytes — enforce strong secrets
	RequestIDMaxLength                 = 128
	IdempotencyKeyMaxLength            = 64
	UserIDMaxLength                    = 128
)
