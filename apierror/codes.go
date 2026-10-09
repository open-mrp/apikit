package apierror

import "net/http"

// Built-in codes. Apps add their own with Register.
const (
	// Authentication and authorization.
	CodeInvalidCredentials      Code = "invalid_credentials" // #nosec G101 - an error code, not a credential
	CodeExpiredToken            Code = "expired_token"       // #nosec G101 - an error code, not a credential
	CodeInsufficientPermissions Code = "insufficient_permissions"

	// Validation. A 422 carries validation_failed at the top level and one entry per failing field in errors; missing_field and invalid_format are the codes of those entries.
	CodeValidationFailed Code = "validation_failed"
	CodeMissingField     Code = "missing_field"
	CodeInvalidFormat    Code = "invalid_format"

	// Query and path parameters.
	CodeParameterMissing    Code = "parameter_missing"
	CodeParameterInvalid    Code = "parameter_invalid"
	CodeParameterUnknown    Code = "parameter_unknown"
	CodeParametersExclusive Code = "parameters_exclusive"

	// The request itself.
	CodeMethodNotAllowed Code = "method_not_allowed"
	CodeRequestTooLarge  Code = "request_too_large"

	// Resources.
	CodeResourceNotFound Code = "resource_not_found"
	CodeResourceExists   Code = "resource_exists"
	CodeResourceConflict Code = "resource_conflict"
	CodeResourceInUse    Code = "resource_in_use"
	CodeResourceGone     Code = "resource_gone"

	// Idempotency.
	CodeIdempotencyInProgress Code = "idempotency_in_progress"
	CodeIdempotencyKeyReused  Code = "idempotency_key_reused"

	// Rate limiting.
	CodeRateLimited Code = "rate_limited"

	// Server failures.
	CodeInternalError        Code = "internal_error"
	CodeServiceUnavailable   Code = "service_unavailable"
	CodeExternalServiceError Code = "external_service_error"
	CodeConnectionError      Code = "connection_error"
	CodeTimeout              Code = "timeout"
	CodeRequestTimeout       Code = "request_timeout"
	CodeClientClosedRequest  Code = "client_closed_request"

	// API versions.
	CodeAPIVersionRequired Code = "api_version_required"
	CodeAPIVersionInvalid  Code = "api_version_invalid"
	CodeAPIVersionTooOld   Code = "api_version_too_old"
)

// StatusClientClosedRequest is nginx's status for a client that disconnected before the response. It is only ever logged: the client is gone.
const StatusClientClosedRequest = 499

func init() {
	for _, s := range []Spec{
		{Code: CodeInvalidCredentials, Type: TypeInvalidRequest, Status: http.StatusUnauthorized,
			Description: "The request's credentials are missing or wrong. Send a valid API key or token."},
		{Code: CodeExpiredToken, Type: TypeInvalidRequest, Status: http.StatusUnauthorized,
			Description: "The access token has expired. Refresh it and retry with the new token."},
		{Code: CodeInsufficientPermissions, Type: TypeInvalidRequest, Status: http.StatusForbidden,
			Description: "The caller is authenticated but not allowed to do this. Use credentials with the required permission."},

		{Code: CodeValidationFailed, Type: TypeInvalidRequest, Status: http.StatusUnprocessableEntity,
			Description: "The request body is well-formed but invalid. errors lists every failing field."},
		{Code: CodeMissingField, Type: TypeInvalidRequest, Status: http.StatusUnprocessableEntity,
			Description: "A required field was not sent. Appears in errors on a validation_failed error."},
		{Code: CodeInvalidFormat, Type: TypeInvalidRequest, Status: http.StatusUnprocessableEntity,
			Description: "A field's value is not in the expected format or range. Appears in errors on a validation_failed error."},

		{Code: CodeParameterMissing, Type: TypeInvalidRequest, Status: http.StatusBadRequest,
			Description: "A required query or path parameter was not sent."},
		{Code: CodeParameterInvalid, Type: TypeInvalidRequest, Status: http.StatusBadRequest,
			Description: "A query or path parameter, or the request body, could not be parsed."},
		{Code: CodeParameterUnknown, Type: TypeInvalidRequest, Status: http.StatusBadRequest,
			Description: "The request sent a parameter or field this endpoint does not accept. Check the spelling against the reference."},
		{Code: CodeParametersExclusive, Type: TypeInvalidRequest, Status: http.StatusBadRequest,
			Description: "Two parameters that cannot be combined were both sent. Send only one of them."},

		{Code: CodeMethodNotAllowed, Type: TypeInvalidRequest, Status: http.StatusMethodNotAllowed,
			Description: "The endpoint exists but does not support this HTTP method."},
		{Code: CodeRequestTooLarge, Type: TypeInvalidRequest, Status: http.StatusRequestEntityTooLarge,
			Description: "The request body is larger than the endpoint accepts. Split the work across several requests."},

		{Code: CodeResourceNotFound, Type: TypeInvalidRequest, Status: http.StatusNotFound,
			Description: "No resource exists with that ID, or the caller cannot see it."},
		{Code: CodeResourceExists, Type: TypeInvalidRequest, Status: http.StatusConflict,
			Description: "A resource with the same unique value already exists."},
		{Code: CodeResourceConflict, Type: TypeInvalidRequest, Status: http.StatusConflict,
			Description: "The resource's current state does not allow this change. Retrieve it and retry against its current state."},
		{Code: CodeResourceInUse, Type: TypeInvalidRequest, Status: http.StatusConflict,
			Description: "The resource cannot be deleted while other records reference it. Archive it instead."},
		{Code: CodeResourceGone, Type: TypeInvalidRequest, Status: http.StatusGone,
			Description: "The resource existed but has been deleted."},

		{Code: CodeIdempotencyInProgress, Type: TypeIdempotency, Status: http.StatusConflict, Transient: true,
			Description: "A request with the same Idempotency-Key is still being processed. Retry after it finishes."},
		{Code: CodeIdempotencyKeyReused, Type: TypeIdempotency, Status: http.StatusUnprocessableEntity,
			Description: "The Idempotency-Key was already used with a different request. Use a new key for a new request."},

		{Code: CodeRateLimited, Type: TypeInvalidRequest, Status: http.StatusTooManyRequests, Transient: true,
			Description: "Too many requests. Wait for the time in the RateLimit-Reset header, then retry."},

		{Code: CodeInternalError, Type: TypeAPI, Status: http.StatusInternalServerError, Transient: true,
			Description: "Something went wrong on the server. Retrying is safe with an Idempotency-Key."},
		{Code: CodeServiceUnavailable, Type: TypeAPI, Status: http.StatusServiceUnavailable, Transient: true,
			Description: "The service is temporarily unavailable. Retry with backoff."},
		{Code: CodeExternalServiceError, Type: TypeAPI, Status: http.StatusBadGateway, Transient: true,
			Description: "A third-party service the request depends on failed. Retry with backoff."},
		{Code: CodeConnectionError, Type: TypeAPI, Status: http.StatusBadGateway, Transient: true,
			Description: "The server could not reach a service the request depends on. Retry with backoff."},
		{Code: CodeTimeout, Type: TypeAPI, Status: http.StatusGatewayTimeout, Transient: true,
			Description: "An operation the request depends on took too long. Retry with backoff."},
		{Code: CodeRequestTimeout, Type: TypeAPI, Status: http.StatusGatewayTimeout, Transient: true,
			Description: "The request took longer than the server allows. Retry with backoff."},
		{Code: CodeClientClosedRequest, Type: TypeAPI, Status: StatusClientClosedRequest,
			Description: "The client disconnected before the server responded. Only appears in request logs."},

		{Code: CodeAPIVersionRequired, Type: TypeInvalidRequest, Status: http.StatusBadRequest,
			Description: "The request has no API version header. Send one with the version your integration was built against."},
		{Code: CodeAPIVersionInvalid, Type: TypeInvalidRequest, Status: http.StatusBadRequest,
			Description: "The API version header names a version that does not exist."},
		{Code: CodeAPIVersionTooOld, Type: TypeInvalidRequest, Status: http.StatusBadRequest,
			Description: "The endpoint does not exist in the requested API version. Upgrade to the version named in the message."},
	} {
		Register(s)
	}
}
