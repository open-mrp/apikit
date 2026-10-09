package apierror

import (
	"bytes"
	"encoding/json"
)

// One field that failed validation.
type FieldErrorObject struct {
	// The request field that failed, e.g. "lines[2].quantity". Null when the failure is not tied to one field.
	Param *string `json:"param"`
	// A machine-readable code for this field's failure.
	Code Code `json:"code"`
	// A human-readable explanation of this field's failure.
	Message string `json:"message"`
}

// What went wrong with a request.
//
// Bulk rows and async job results report their failures with this same object.
type ErrorObject struct {
	// The class of error.
	Type Type `json:"type"`
	// A machine-readable code for the error.
	Code Code `json:"code"`
	// A human-readable explanation of the error.
	Message string `json:"message"`
	// The request parameter or field the error is about; on a validation error, the first of errors. Null when the error is not about one.
	Param *string `json:"param"`
	// Whether the same request may succeed if retried.
	IsTransient bool `json:"is_transient"`
	// Every failing field of a validation error. Empty for other errors.
	Errors []FieldErrorObject `json:"errors"`
	// The docs page for this code.
	DocURL *string `json:"doc_url"`

	// details is the code's extra member, written under detailsField when the code declares one.
	details      any
	detailsField string
}

// MarshalJSON writes the fixed members, then the code's details member when it declares one (null when no details were attached).
func (o ErrorObject) MarshalJSON() ([]byte, error) {
	type fixed ErrorObject
	f := fixed(o)
	if f.Errors == nil {
		f.Errors = []FieldErrorObject{}
	}
	b, err := json.Marshal(f)
	if err != nil || o.detailsField == "" {
		return b, err
	}
	key, err := json.Marshal(o.detailsField)
	if err != nil {
		return nil, err
	}
	val, err := json.Marshal(o.details)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.Write(b[:len(b)-1])
	buf.WriteByte(',')
	buf.Write(key)
	buf.WriteByte(':')
	buf.Write(val)
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// Details returns the code's extra member and its name, or "" when the code declares none.
func (o ErrorObject) Details() (field string, value any) {
	return o.detailsField, o.details
}

// SchemaExample returns a representative error object for schema generation.
func (ErrorObject) SchemaExample() any {
	param := "email"
	return ErrorObject{
		Type:        TypeInvalidRequest,
		Code:        CodeValidationFailed,
		Message:     "email must be a valid email address.",
		Param:       &param,
		IsTransient: false,
		Errors:      []FieldErrorObject{{Param: &param, Code: CodeInvalidFormat, Message: "email must be a valid email address."}},
	}
}

// The body of every error response.
type Response struct {
	// What went wrong.
	Error ErrorObject `json:"error"`
}

// SchemaExample returns a representative error response for schema generation.
func (Response) SchemaExample() any {
	return Response{Error: ErrorObject{}.SchemaExample().(ErrorObject)}
}

// Object renders the client-facing error object.
func (e *APIError) Object() ErrorObject {
	if e == nil {
		return ErrorObject{Errors: []FieldErrorObject{}}
	}
	obj := ErrorObject{
		Type:        e.Type,
		Code:        e.Code,
		Message:     e.PublicMessage,
		IsTransient: e.IsTransient,
		Errors:      make([]FieldErrorObject, len(e.Errors)),
	}
	if e.Param != "" {
		obj.Param = &e.Param
	}
	for i, fe := range e.Errors {
		obj.Errors[i] = FieldErrorObject{Code: fe.Code, Message: fe.Message}
		if fe.Param != "" {
			obj.Errors[i].Param = &fe.Param
		}
	}
	if url := e.Code.DocURL(); url != "" {
		obj.DocURL = &url
	}
	if spec, ok := Lookup(e.Code); ok && spec.DetailsField != "" {
		obj.detailsField = spec.DetailsField
		obj.details = e.Details
	}
	return obj
}

// Response renders the full error response body.
func (e *APIError) Response() Response {
	return Response{Error: e.Object()}
}
