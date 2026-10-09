// Package apierror defines the error every API response and service call reports.
//
// An APIError separates what the client sees (the forge.1 error object: type, code, message, param, is_transient, errors, doc_url) from what only logs and traces see (InternalMessage, Internal, Stack). Everything fixed about a code, such as its HTTP status and whether it is transient, comes from the code's registered Spec, so an error cannot be built with a status that disagrees with its code.
package apierror

import (
	"errors"
	"fmt"
	"runtime"
)

// FieldError is one failing field of a validation_failed error.
type FieldError struct {
	// Param is the field's path in the request, e.g. "lines[2].quantity". Empty when the failure is not tied to one field.
	Param string
	// Code is the specific failure, usually CodeMissingField or CodeInvalidFormat.
	Code Code
	// Message is the human-readable explanation.
	Message string
}

// Field returns a FieldError.
func Field(param string, code Code, message string) FieldError {
	return FieldError{Param: param, Code: code, Message: message}
}

// APIError is the error type used throughout an API. Build one with a constructor (NewValidationError, NewNotFoundError, ...) or New, never as a literal, so its type and transience match its code.
type APIError struct {
	// Code is the registered code; its Spec fixes the type, status and transience.
	Code Code
	// Type is the code's type, copied from its Spec.
	Type Type
	// PublicMessage is returned to the client.
	PublicMessage string
	// Param names the request parameter or field the error is about, such as a malformed query parameter or a duplicate value. Empty when it is not about one. New sets it to the first failing field when Errors is given without it.
	Param string
	// Errors lists every failing field of a validation_failed error, and is empty otherwise.
	Errors []FieldError
	// IsTransient is the code's transience, copied from its Spec.
	IsTransient bool
	// Details is the extra member for a code that declares a DetailsField, such as an app's quota information. Nil otherwise.
	Details any
	// InternalMessage is for logs and traces. Never sent to clients.
	InternalMessage string
	// Internal is the underlying error, if any. Never sent to clients; reachable through errors.Unwrap.
	Internal error
	// Stack is the goroutine stack where a 5xx error was created, so a trace points at the failing code rather than the response writer. Never sent to clients.
	Stack string
}

// Option adjusts an APIError as New builds it.
type Option func(*APIError)

// WithInternal wraps the underlying error for logs and traces.
func WithInternal(err error) Option {
	return func(e *APIError) { e.Internal = err }
}

// WithInternalMessage sets the developer-facing message for logs and traces.
func WithInternalMessage(msg string) Option {
	return func(e *APIError) { e.InternalMessage = msg }
}

// WithParam names the request parameter or field the error is about.
func WithParam(param string) Option {
	return func(e *APIError) { e.Param = param }
}

// WithFieldErrors attaches the failing fields of a validation error.
func WithFieldErrors(errs ...FieldError) Option {
	return func(e *APIError) { e.Errors = append(e.Errors, errs...) }
}

// WithDetails attaches the extra member of a code that declares a DetailsField.
func WithDetails(details any) Option {
	return func(e *APIError) { e.Details = details }
}

// New builds an error for any registered code. It panics on an unregistered code, which is a programming error. Prefer the named constructors for built-in codes.
func New(code Code, publicMessage string, opts ...Option) *APIError {
	spec, ok := Lookup(code)
	if !ok {
		panic(fmt.Sprintf("apierror: code %q is not registered", code))
	}
	e := &APIError{
		Code:          code,
		Type:          spec.Type,
		PublicMessage: publicMessage,
		IsTransient:   spec.Transient,
	}
	for _, opt := range opts {
		opt(e)
	}
	if e.Param == "" && len(e.Errors) > 0 {
		e.Param = e.Errors[0].Param
	}
	e.InternalMessage = nestInternalMessage(e.Internal, e.InternalMessage)
	if spec.Status >= 500 {
		// An error relayed from a downstream service keeps the stack captured where it began.
		if inner, ok := e.Internal.(*APIError); ok && inner.Stack != "" {
			e.Stack = inner.Stack
		} else {
			e.Stack = captureStack()
		}
	}
	return e
}

// Error returns the internal message, with the root cause appended, for logs. It never includes the public message, so a nested error that only carries a public message adds nothing to its parent's text; use Describe when reporting an error.
func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	// Wrapped APIErrors' internal messages are already chained into this one by New, so only the first cause that is not an APIError adds anything.
	cause := e.Internal
	for {
		inner, ok := cause.(*APIError)
		if !ok || inner == nil {
			break
		}
		cause = inner.Internal
	}
	if cause == nil || cause.Error() == "" {
		return e.InternalMessage
	}
	if e.InternalMessage == "" {
		return cause.Error()
	}
	return e.InternalMessage + ": " + cause.Error()
}

// Unwrap returns the underlying error for errors.Is and errors.As.
func (e *APIError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Internal
}

// Status returns the HTTP status of the error's code.
func (e *APIError) Status() int {
	return e.Code.Status()
}

// Describe renders an error for a log line or a stored failure record, and never returns "" for a non-nil error. Error() is deliberately internal-only, so an APIError built with just a public message (most 4xx constructors) has an empty Error(); reporting it with Error() would record that something failed but not what.
func Describe(err error) string {
	if err == nil {
		return ""
	}
	if text := err.Error(); text != "" {
		return text
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr != nil {
		switch {
		case apiErr.PublicMessage != "":
			return string(apiErr.Code) + ": " + apiErr.PublicMessage
		case apiErr.Code != "":
			return string(apiErr.Code)
		}
	}
	return fmt.Sprintf("%T with no message", err)
}

// HasCode reports whether err is or wraps an APIError with the given code.
func HasCode(err error, code Code) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr != nil && apiErr.Code == code
}

// nestInternalMessage chains a wrapped APIError's internal message onto the outer one, so a failure relayed through several layers reads "outer: inner".
func nestInternalMessage(internal error, msg string) string {
	nested, ok := internal.(*APIError)
	if !ok || nested.InternalMessage == "" {
		return msg
	}
	if msg == "" {
		return nested.InternalMessage
	}
	return msg + ": " + nested.InternalMessage
}

func captureStack() string {
	buf := make([]byte, 32<<10)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}
