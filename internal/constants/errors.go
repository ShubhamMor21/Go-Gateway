package constants

// Machine-readable error codes returned in JSON responses.
// Frontend/consumers should switch on these codes, never on message strings.
const (
	ErrCodeUnauthorized      = "UNAUTHORIZED"
	ErrCodeForbidden         = "FORBIDDEN"
	ErrCodeNotFound          = "NOT_FOUND"
	ErrCodeBadRequest        = "BAD_REQUEST"
	ErrCodeValidation        = "VALIDATION_ERROR"
	ErrCodeInternalError     = "INTERNAL_ERROR"
	ErrCodeRateLimitExceeded = "RATE_LIMIT_EXCEEDED"
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	ErrCodeCircuitOpen       = "CIRCUIT_BREAKER_OPEN"
	ErrCodeTimeout           = "REQUEST_TIMEOUT"
	ErrCodeRequestTooLarge   = "REQUEST_TOO_LARGE"
	ErrCodeTokenExpired      = "TOKEN_EXPIRED"
	ErrCodeTokenInvalid      = "TOKEN_INVALID"
	ErrCodeMissingToken      = "MISSING_TOKEN"
	ErrCodeCacheError        = "CACHE_ERROR"
	ErrCodeQueueError        = "QUEUE_ERROR"
	ErrCodeGRPCError         = "GRPC_ERROR"
	ErrCodeIdempotency       = "MISSING_IDEMPOTENCY_KEY"
	ErrCodeInvalidIdempotency = "INVALID_IDEMPOTENCY_KEY"
	ErrCodeForbiddenRole     = "INSUFFICIENT_PERMISSIONS"
	ErrCodeInvalidContentType = "INVALID_CONTENT_TYPE"
	ErrCodeInvalidRequestID  = "INVALID_REQUEST_ID"
)
