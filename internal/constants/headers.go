package constants

// HTTP header names and Fiber locals keys.
const (
	// Tracing / propagation
	HeaderRequestID     = "X-Request-ID"
	HeaderCorrelationID = "X-Correlation-ID"
	HeaderTraceID       = "X-Trace-ID"

	// Identity (injected by auth middleware, consumed by downstream)
	HeaderUserID   = "X-User-ID"
	HeaderUserRole = "X-User-Role"

	// Standard
	HeaderContentType    = "Content-Type"
	HeaderAuthorization  = "Authorization"
	HeaderIdempotencyKey = "Idempotency-Key"

	// Security response headers
	HeaderCSP               = "Content-Security-Policy"
	HeaderHSTS              = "Strict-Transport-Security"
	HeaderXFrameOptions     = "X-Frame-Options"
	HeaderXContentType      = "X-Content-Type-Options"
	HeaderXXSSProtection    = "X-XSS-Protection"
	HeaderReferrerPolicy    = "Referrer-Policy"
	HeaderPermissionsPolicy = "Permissions-Policy"

	// Fiber locals keys (c.Locals) — used to pass values through the middleware chain
	LocalRequestID = "request_id"
	LocalUserID    = "user_id"
	LocalUserRole  = "user_role"
	LocalLogger    = "logger"

	// Token revocation locals — set by Auth middleware, read by Logout handler
	LocalTokenHash = "token_hash" // SHA-256(raw_token) hex string
	LocalTokenExp  = "token_exp"  // time.Time of token expiry, for revocation TTL
)
