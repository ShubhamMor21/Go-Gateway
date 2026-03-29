package constants

// HTTP header names. Always reference these constants — never write raw header strings.
const (
	// Tracing / propagation
	HeaderRequestID     = "X-Request-ID"
	HeaderCorrelationID = "X-Correlation-ID"
	HeaderTraceID       = "X-Trace-ID"

	// Identity (injected by auth middleware, consumed by downstream)
	HeaderUserID   = "X-User-ID"
	HeaderUserRole = "X-User-Role"

	// Standard
	HeaderContentType   = "Content-Type"
	HeaderAuthorization = "Authorization"
	HeaderIdempotencyKey = "Idempotency-Key"

	// Security response headers
	HeaderCSP              = "Content-Security-Policy"
	HeaderHSTS             = "Strict-Transport-Security"
	HeaderXFrameOptions    = "X-Frame-Options"
	HeaderXContentType     = "X-Content-Type-Options"
	HeaderXXSSProtection   = "X-XSS-Protection"
	HeaderReferrerPolicy   = "Referrer-Policy"
	HeaderPermissionsPolicy = "Permissions-Policy"

	// Fiber locals keys (c.Locals)
	LocalRequestID = "request_id"
	LocalUserID    = "user_id"
	LocalUserRole  = "user_role"
	LocalLogger    = "logger"
)
