package constants

// Machine-readable error codes returned in JSON responses.
const (
	ErrCodeUnauthorized       = "UNAUTHORIZED"
	ErrCodeForbidden          = "FORBIDDEN"
	ErrCodeForbiddenOwnership = "FORBIDDEN_OWNERSHIP" // user accessing another user's resource
	ErrCodeForbiddenRole      = "INSUFFICIENT_PERMISSIONS"
	ErrCodeNotFound           = "NOT_FOUND"
	ErrCodeBadRequest         = "BAD_REQUEST"
	ErrCodeValidation         = "VALIDATION_ERROR"
	ErrCodeInternalError      = "INTERNAL_ERROR"
	ErrCodeRateLimitExceeded  = "RATE_LIMIT_EXCEEDED"
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	ErrCodeCircuitOpen        = "CIRCUIT_BREAKER_OPEN"
	ErrCodeTimeout            = "REQUEST_TIMEOUT"
	ErrCodeRequestTooLarge    = "REQUEST_TOO_LARGE"
	ErrCodeTokenExpired       = "TOKEN_EXPIRED"
	ErrCodeTokenInvalid       = "TOKEN_INVALID"
	ErrCodeTokenRevoked       = "TOKEN_REVOKED"
	ErrCodeMissingToken       = "MISSING_TOKEN"
	ErrCodeIPBlocked          = "IP_BLOCKED"
	ErrCodeCacheError         = "CACHE_ERROR"
	ErrCodeGRPCError          = "GRPC_ERROR"
	ErrCodeIdempotency        = "MISSING_IDEMPOTENCY_KEY"
	ErrCodeInvalidIdempotency = "INVALID_IDEMPOTENCY_KEY"
	ErrCodeInvalidContentType = "INVALID_CONTENT_TYPE"
	ErrCodeInvalidRequestID   = "INVALID_REQUEST_ID"
	ErrCodeQueueError         = "QUEUE_ERROR"
)
