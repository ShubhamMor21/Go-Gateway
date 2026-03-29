package constants

// User-facing and system messages. All log/response messages must reference these — no inline strings.
const (
	// Auth
	MsgUnauthorized      = "Unauthorized"
	MsgForbidden         = "Forbidden"
	MsgMissingAuthHeader = "Missing authorization header"
	MsgTokenExpired      = "Token has expired"
	MsgTokenInvalid      = "Token is invalid"

	// Request lifecycle
	MsgRequestReceived  = "Request received"
	MsgRequestCompleted = "Request completed"
	MsgBadRequest       = "Bad request"
	MsgInvalidPayload   = "Invalid request payload"
	MsgNotFound         = "Not found"

	// Rate limiting / load shedding
	MsgRateLimitExceeded  = "Rate limit exceeded, please retry after a moment"
	MsgServiceUnavailable = "Service temporarily unavailable, please retry later"

	// Circuit breaker
	MsgCircuitOpen = "Downstream service is temporarily unavailable"

	// Cache
	MsgCacheHit  = "Cache hit"
	MsgCacheMiss = "Cache miss"

	// Queue
	MsgPaymentQueued    = "Payment queued successfully"
	MsgQueuePublishFail = "Failed to publish message to queue"

	// gRPC
	MsgGRPCCallFailed = "gRPC call failed"
	MsgGRPCTimeout    = "gRPC call timed out"

	// Health
	MsgHealthOK       = "Gateway is healthy"
	MsgHealthDegraded = "Gateway is degraded"

	// Internal errors
	MsgInternalError = "An internal error occurred"
	MsgRequestTimeout = "Request timed out"

	// Idempotency
	MsgMissingIdempotencyKey  = "Idempotency-Key header is required"
	MsgInvalidIdempotencyKey  = "Idempotency-Key must be a valid UUID (36 characters)"

	// RBAC
	MsgForbiddenRole = "Insufficient permissions for this operation"

	// Content-Type
	MsgInvalidContentType = "Content-Type must be application/json"

	// Request ID
	MsgInvalidRequestID = "X-Request-ID must be alphanumeric with hyphens, max 128 characters"
)
