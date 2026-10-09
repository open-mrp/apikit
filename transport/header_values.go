package transport

const (
	IdempotencyKeyHeader     = "Idempotency-Key"
	IdempotentReplayedHeader = "Idempotent-Replayed"
	ContentTypeHeader        = "Content-Type"
	AuthorizationHeader      = "Authorization"
	RequestIDHeader          = "Request-ID"
	WwwAuthenticateHeader    = "WWW-Authenticate"
	RetryAfterHeader         = "Retry-After"
	RateLimitLimitHeader     = "RateLimit-Limit"
	RateLimitRemainingHeader = "RateLimit-Remaining"
	RateLimitResetHeader     = "RateLimit-Reset"
	LocationHeader           = "Location"
	DeprecationHeader        = "Deprecation"
	SunsetHeader             = "Sunset"
)

// The API version header is named by the app; see version.Header.
