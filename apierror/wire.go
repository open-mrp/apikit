package apierror

import (
	"encoding/json"
	"errors"
	"reflect"
)

// wireError is how an APIError travels between services of one app, such as over gRPC. Unlike the
// client-facing ErrorObject it keeps the internal fields, so logs and traces on the receiving side
// still show what failed where.
type wireError struct {
	Code            Code             `json:"code"`
	Type            Type             `json:"type"`
	PublicMessage   string           `json:"message"`
	Param           string           `json:"param,omitempty"`
	Errors          []wireFieldError `json:"errors,omitempty"`
	IsTransient     bool             `json:"is_transient"`
	Details         json.RawMessage  `json:"details,omitempty"`
	InternalMessage string           `json:"internal_message,omitempty"`
	InternalError   string           `json:"internal_error,omitempty"`
	Stack           string           `json:"stack,omitempty"`
}

type wireFieldError struct {
	Param   string `json:"param,omitempty"`
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

// ToJSON encodes the whole error, internal fields included, for another service of the same app to
// rebuild with FromJSON. It is not a client response: use Response for that.
func (e *APIError) ToJSON() ([]byte, error) {
	if e == nil {
		return nil, nil
	}
	w := wireError{
		Code:            e.Code,
		Type:            e.Type,
		PublicMessage:   e.PublicMessage,
		Param:           e.Param,
		IsTransient:     e.IsTransient,
		InternalMessage: e.InternalMessage,
		Stack:           e.Stack,
	}
	for _, fe := range e.Errors {
		w.Errors = append(w.Errors, wireFieldError(fe))
	}
	if e.Details != nil {
		b, err := json.Marshal(e.Details)
		if err != nil {
			return nil, err
		}
		w.Details = b
	}
	// The internal message already carries those of nested APIErrors, so only the root cause's text
	// is added, which is what Error() appends on this side too.
	if cause := rootCause(e.Internal); cause != nil {
		w.InternalError = cause.Error()
	}
	return json.Marshal(w)
}

// FromJSON rebuilds an error encoded by ToJSON. A registered code's type and transience come from its
// Spec, and its details are decoded into the type of the Spec's DetailsExample; an unregistered code
// keeps what was sent, with details left as raw JSON.
func FromJSON(data []byte) (*APIError, error) {
	var w wireError
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, err
	}
	e := &APIError{
		Code:            w.Code,
		Type:            w.Type,
		PublicMessage:   w.PublicMessage,
		Param:           w.Param,
		IsTransient:     w.IsTransient,
		InternalMessage: w.InternalMessage,
		Stack:           w.Stack,
	}
	for _, fe := range w.Errors {
		e.Errors = append(e.Errors, FieldError(fe))
	}
	if w.InternalError != "" {
		e.Internal = errors.New(w.InternalError)
	}
	spec, registered := Lookup(w.Code)
	if registered {
		e.Type = spec.Type
		e.IsTransient = spec.Transient
	}
	if len(w.Details) > 0 && string(w.Details) != "null" {
		details, err := decodeDetails(spec, registered, w.Details)
		if err != nil {
			return nil, err
		}
		e.Details = details
	}
	return e, nil
}

func decodeDetails(spec Spec, registered bool, raw json.RawMessage) (any, error) {
	if !registered || spec.DetailsExample == nil {
		return raw, nil
	}
	t := reflect.TypeOf(spec.DetailsExample)
	if t.Kind() == reflect.Pointer {
		v := reflect.New(t.Elem())
		if err := json.Unmarshal(raw, v.Interface()); err != nil {
			return nil, err
		}
		return v.Interface(), nil
	}
	v := reflect.New(t)
	if err := json.Unmarshal(raw, v.Interface()); err != nil {
		return nil, err
	}
	return v.Elem().Interface(), nil
}

// rootCause follows nested APIErrors to the first error that is not one.
func rootCause(err error) error {
	for {
		inner, ok := err.(*APIError)
		if !ok {
			return err
		}
		if inner == nil {
			return nil
		}
		err = inner.Internal
	}
}
