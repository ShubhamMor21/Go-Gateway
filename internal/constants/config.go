package constants

// Default/fallback values used ONLY when the corresponding ENV variable is absent.
const (
	DefaultServerPort             = "3030"
	DefaultMetricsPort            = "9090"
	DefaultLogLevel               = "info"
	DefaultRateLimitRequests      = 100
	DefaultRateLimitWindowSeconds = 60
	DefaultCacheTTLSeconds        = 300
	DefaultRequestTimeoutSeconds  = 30
	DefaultMaxRequestSizeMB       = 10
	DefaultGracefulShutdownSeconds = 30
	DefaultCBMaxRequests          = 5
	DefaultCBIntervalSeconds      = 60
	DefaultCBTimeoutSeconds       = 30
	DefaultCBFailureRatio         = 0.6
	DefaultLoadShedMaxConnections = 10_000
	DefaultGRPCMaxRetries         = 3
	DefaultGRPCRetryBaseDelayMs   = 100
	DefaultFiberReadTimeoutSeconds  = 30
	DefaultFiberWriteTimeoutSeconds = 30
	DefaultFiberIdleTimeoutSeconds  = 120
	DefaultFiberConcurrency         = 262_144
	DefaultKafkaRetries             = 3
	DefaultKafkaRetryBackoffMs      = 200
	DefaultKafkaSASLMechanism       = "PLAIN"
	DefaultKafkaTLSEnabled          = false
	DefaultRateLimitFailOpen        = false

	// [HIGH] gRPC should default to TLS-on — plaintext is only acceptable
	// inside a trusted service mesh that provides mTLS at the infra layer.
	DefaultGRPCTLSEnabled = true

	DefaultJWTAlgorithm = "HS256" // HS256 | RS256 | ES256

	// Validation minimums
	JWTSecretMinLength  = 32  // bytes — NIST minimum for HMAC-SHA256
	RequestIDMaxLength  = 128
	UserIDMaxLength     = 128

	// Redis keys — used by cache package for revocation and blocklist
	RedisKeyRevokedPrefix = "revoked:token:" // + SHA256(raw_token_hex)
	RedisKeyBlockedIPs    = "blocked_ips"    // Redis SET of blocked IP strings
)
