package constants

// User-facing and system messages. All log/response messages must reference these.
const (
	// Auth
	MsgUnauthorized      = "Unauthorized"
	MsgForbidden         = "Forbidden"
	MsgMissingAuthHeader = "Missing authorization header"
	MsgTokenExpired      = "Token has expired"
	MsgTokenInvalid      = "Token is invalid"
	MsgTokenRevoked      = "Token has been revoked"

	// Ownership / RBAC
	MsgForbiddenRole      = "Insufficient permissions for this operation"
	MsgForbiddenOwnership = "You are not authorized to access this resource"

	// Request lifecycle
	MsgRequestReceived = "Request received"
	MsgRequestCompleted = "Request completed"
	MsgBadRequest      = "Bad request"
	MsgInvalidPayload  = "Invalid request payload"
	MsgNotFound        = "Not found"

	// Rate limiting / load shedding
	MsgRateLimitExceeded  = "Rate limit exceeded, please retry after a moment"
	MsgServiceUnavailable = "Service temporarily unavailable, please retry later"

	// Circuit breaker
	MsgCircuitOpen = "Downstream service is temporarily unavailable"

	// Cache
	MsgCacheHit  = "Cache hit"
	MsgCacheMiss = "Cache miss"

	// gRPC
	MsgGRPCCallFailed = "gRPC call failed"
	MsgGRPCTimeout    = "gRPC call timed out"

	// Health
	MsgHealthOK       = "Gateway is healthy"
	MsgHealthDegraded = "Gateway is degraded"

	// Internal
	MsgInternalError  = "An internal error occurred"
	MsgRequestTimeout = "Request timed out"

	// IP blocklist
	MsgIPBlocked = "Access denied"

	// Queue
	MsgPaymentQueued    = "Event queued successfully"
	MsgQueuePublishFail = "Failed to publish message to queue"

	// Logout
	MsgLogoutSuccess = "Logged out successfully"

	// Validation
	MsgInvalidContentType = "Content-Type must be application/json"
	MsgInvalidRequestID   = "X-Request-ID must be alphanumeric with hyphens, max 128 characters"

	// Idempotency
	MsgMissingIdempotencyKey = "Idempotency-Key header is required"
	MsgInvalidIdempotencyKey = "Idempotency-Key must be a valid UUID (36 characters)"
)
