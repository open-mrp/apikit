package apierror

import (
	"fmt"
	"strings"
)

// NewValidationError is a 422 listing every failing field; its param is the first of them. Collect all failures before returning one, so the client can fix them in a single round trip.
func NewValidationError(message string, errs ...FieldError) *APIError {
	return New(CodeValidationFailed, message, WithFieldErrors(errs...))
}

// NewFieldError is a 422 for a single failing field.
func NewFieldError(param string, code Code, message string) *APIError {
	return NewValidationError(message, Field(param, code, message))
}

// NewMissingFieldError is a 422 for a required field that was not sent.
func NewMissingFieldError(param, message string) *APIError {
	return NewFieldError(param, CodeMissingField, message)
}

// NewInvalidFormatError is a 422 for a field whose value has the wrong format or range.
func NewInvalidFormatError(param, message string) *APIError {
	return NewFieldError(param, CodeInvalidFormat, message)
}

// NewParameterMissingError is a 400 for a required query or path parameter that was not sent.
func NewParameterMissingError(param, message string) *APIError {
	return New(CodeParameterMissing, message, WithParam(param))
}

// NewParameterInvalidError is a 400 for a query or path parameter, or a body, that could not be parsed.
func NewParameterInvalidError(param, message string) *APIError {
	return New(CodeParameterInvalid, message, WithParam(param))
}

// NewParameterUnknownError is a 400 for a parameter or field the endpoint does not accept.
func NewParameterUnknownError(param, message string) *APIError {
	return New(CodeParameterUnknown, message, WithParam(param))
}

// NewParametersExclusiveError is a 400 for two parameters that cannot be combined. param is the one the caller should drop.
func NewParametersExclusiveError(param, message string) *APIError {
	return New(CodeParametersExclusive, message, WithParam(param))
}

// NewAuthenticationError is a 401 for missing or wrong credentials.
func NewAuthenticationError(message string) *APIError {
	return New(CodeInvalidCredentials, message)
}

// NewExpiredTokenError is a 401 for an expired token, kept apart from NewAuthenticationError so routine token refreshes can be told from bad credentials.
func NewExpiredTokenError(message string) *APIError {
	return New(CodeExpiredToken, message)
}

// NewAuthorizationError is a 403 for a caller without the required permission.
func NewAuthorizationError(message string) *APIError {
	return New(CodeInsufficientPermissions, message)
}

// NewNotFoundError is a 404 for a resource that does not exist or the caller cannot see.
func NewNotFoundError(message string) *APIError {
	return New(CodeResourceNotFound, message)
}

// NewExistsError is a 409 for a duplicate of a unique value. param names the field holding it.
func NewExistsError(param, message string) *APIError {
	return New(CodeResourceExists, message, WithParam(param))
}

// NewConflictError is a 409 for a change the resource's current state does not allow.
func NewConflictError(message string) *APIError {
	return New(CodeResourceConflict, message)
}

// NewConflictErrorWithParam is a 409 for a change one field's value does not allow, such as a username already taken.
func NewConflictErrorWithParam(param, message string) *APIError {
	return New(CodeResourceConflict, message, WithParam(param))
}

// NewInUseError is a 409 for deleting a resource other records still reference. The message should point to archiving.
func NewInUseError(message string) *APIError {
	return New(CodeResourceInUse, message)
}

// NewGoneError is a 410 for a resource that has been deleted.
func NewGoneError(message string) *APIError {
	return New(CodeResourceGone, message)
}

// NewIdempotencyInProgressError is a 409 while the first request with this key is still running.
func NewIdempotencyInProgressError(key string) *APIError {
	return New(CodeIdempotencyInProgress, fmt.Sprintf("A request with the Idempotency-Key %q is still being processed.", key))
}

// NewIdempotencyKeyReusedError is a 422 for a key already used with a different request.
func NewIdempotencyKeyReusedError(key string) *APIError {
	return New(CodeIdempotencyKeyReused, fmt.Sprintf("The Idempotency-Key %q was already used with a different request. Use a new key.", key))
}

// NewRateLimitedError is a 429.
func NewRateLimitedError(message string) *APIError {
	return New(CodeRateLimited, message)
}

// NewMethodNotAllowedError is a 405.
func NewMethodNotAllowedError(message string) *APIError {
	return New(CodeMethodNotAllowed, message)
}

// NewRequestTooLargeError is a 413 for a body over the endpoint's limit.
func NewRequestTooLargeError(message string) *APIError {
	return New(CodeRequestTooLarge, message)
}

// internalPublicMessage is all a client learns about a server failure.
const internalPublicMessage = "Something went wrong."

// NewInternalError is a 500 wrapping the underlying failure for logs. The client sees only a generic message.
func NewInternalError(err error, internalMessage string) *APIError {
	return New(CodeInternalError, internalPublicMessage, WithInternal(err), WithInternalMessage(internalMessage))
}

// NewInvariantViolationError is a 500 for a condition correct code never reaches, such as a row missing right after it was read.
func NewInvariantViolationError(internalMessage string) *APIError {
	return New(CodeInternalError, internalPublicMessage, WithInternalMessage(internalMessage))
}

// NewServiceUnavailableError is a 503 for a dependency that is down.
func NewServiceUnavailableError(err error, internalMessage string) *APIError {
	return New(CodeServiceUnavailable, "The service is temporarily unavailable.", WithInternal(err), WithInternalMessage(internalMessage))
}

// NewExternalServiceError is a 502 for a failing third-party service.
func NewExternalServiceError(err error, internalMessage string) *APIError {
	return New(CodeExternalServiceError, "A service this request depends on failed.", WithInternal(err), WithInternalMessage(internalMessage))
}

// NewRequestTimeoutError is a 504: the server ran out of time, so it is a server failure rather than a 408.
func NewRequestTimeoutError(internalMessage string) *APIError {
	return New(CodeRequestTimeout, "The request timed out.", WithInternalMessage(internalMessage))
}

// NewClientClosedRequestError is a 499 for a client that disconnected before the response.
func NewClientClosedRequestError() *APIError {
	return New(CodeClientClosedRequest, "The client closed the request.", WithInternalMessage("client closed request"))
}

// NewAPIVersionRequiredError is a 400 for a request without the version header.
func NewAPIVersionRequiredError(header, latest string) *APIError {
	return New(CodeAPIVersionRequired, fmt.Sprintf("The %s header is required. The latest version is %s.", header, latest), WithParam(header))
}

// NewAPIVersionInvalidError is a 400 for a version that does not exist.
func NewAPIVersionInvalidError(header, requested string, supported []string) *APIError {
	return New(CodeAPIVersionInvalid, fmt.Sprintf("API version %q does not exist. Supported versions: %s.", requested, strings.Join(supported, ", ")), WithParam(header))
}

// NewAPIVersionTooOldError is a 400 for an endpoint added after the requested version.
func NewAPIVersionTooOldError(requested, minimum string) *APIError {
	return New(CodeAPIVersionTooOld, fmt.Sprintf("This endpoint requires API version %s or newer. You requested %s.", minimum, requested))
}
