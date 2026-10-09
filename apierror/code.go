package apierror

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
)

// Code is the machine-readable identifier of an error condition. Codes are permanent once public: clients branch on them, and each has a docs page.
type Code string

// Type is the broad class of an error, so a client can handle a family of codes it does not know individually.
type Type string

const (
	// TypeAPI covers server-side failures (5xx).
	TypeAPI Type = "api_error"
	// TypeIdempotency covers idempotency key conflicts.
	TypeIdempotency Type = "idempotency_error"
	// TypeInvalidRequest covers client mistakes: bad input, missing auth, a resource that does not exist. The client must change the request before resending.
	TypeInvalidRequest Type = "invalid_request_error"
)

// RegisterType adds an app-defined error type, for a family of codes that clients should handle together. Call it during startup, before registering the codes that use it. It panics on an invalid or duplicate type.
func RegisterType(t Type) {
	if t == "" || strings.ToLower(string(t)) != string(t) || strings.ContainsAny(string(t), " -.") {
		panic(fmt.Sprintf("apierror: type %q must be non-empty snake_case", t))
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if types[t] {
		panic(fmt.Sprintf("apierror: type %q is already registered", t))
	}
	types[t] = true
}

// IsValid reports whether t is a built-in or registered error type.
func (t Type) IsValid() bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return types[t]
}

// EnumValues lists every error type, sorted, for schema generation.
func (t Type) EnumValues() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(types))
	for typ := range types {
		out = append(out, string(typ))
	}
	slices.Sort(out)
	return out
}

// Spec declares everything fixed about a code: its type, HTTP status, whether retrying can succeed, and the docs a client reads about it.
type Spec struct {
	Code Code
	Type Type
	// Status is the HTTP status the code is returned with.
	Status int
	// Transient reports that the same request may succeed if retried.
	Transient bool
	// Description says when the code is returned and what the caller should do, for the error-code reference.
	Description string
	// DetailsField (optional) names an extra member the error object carries for this code, such as "quota". Errors with this code always include it, null when no details were attached.
	DetailsField string
	// DetailsExample (optional) is a representative value of the details, used to document their schema.
	DetailsExample any
}

var (
	registryMu sync.RWMutex
	registry   = map[Code]Spec{}
	types      = map[Type]bool{TypeAPI: true, TypeIdempotency: true, TypeInvalidRequest: true}
)

// Register adds an app-defined code to the built-in ones (codes.go). Call it during startup. It panics on an invalid spec or a code that is already registered, since either is a programming error that would otherwise surface as a wrong response.
func Register(spec Spec) {
	if err := spec.validate(); err != nil {
		panic(err)
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registry[spec.Code]; exists {
		panic(fmt.Sprintf("apierror: code %q is already registered", spec.Code))
	}
	registry[spec.Code] = spec
}

func (s Spec) validate() error {
	switch {
	case s.Code == "" || strings.ToLower(string(s.Code)) != string(s.Code) || strings.ContainsAny(string(s.Code), " -."):
		return fmt.Errorf("apierror: code %q must be non-empty snake_case", s.Code)
	case !s.Type.IsValid():
		return fmt.Errorf("apierror: code %q has unknown type %q", s.Code, s.Type)
	case s.Status < 400 || s.Status > 599:
		return fmt.Errorf("apierror: code %q has non-error status %d", s.Code, s.Status)
	case s.Description == "":
		return fmt.Errorf("apierror: code %q needs a description for its docs page", s.Code)
	case s.DetailsExample != nil && s.DetailsField == "":
		return fmt.Errorf("apierror: code %q has a details example but no details field", s.Code)
	}
	return nil
}

// Lookup returns the spec registered for code.
func Lookup(code Code) (Spec, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	s, ok := registry[code]
	return s, ok
}

// Specs returns every registered code, sorted by code, for the error-code reference and schema generation.
func Specs() []Spec {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]Spec, 0, len(registry))
	for _, s := range registry {
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b Spec) int { return strings.Compare(string(a.Code), string(b.Code)) })
	return out
}

// IsValid reports whether c is registered.
func (c Code) IsValid() bool {
	_, ok := Lookup(c)
	return ok
}

// EnumValues lists every registered code, for schema generation.
func (c Code) EnumValues() []string {
	specs := Specs()
	out := make([]string, len(specs))
	for i, s := range specs {
		out[i] = string(s.Code)
	}
	return out
}

// Status returns the HTTP status for c, or 500 for an unregistered code.
func (c Code) Status() int {
	if s, ok := Lookup(c); ok {
		return s.Status
	}
	return http.StatusInternalServerError
}

var docURLFunc func(Code) string

// SetDocURL sets how an error's doc_url is built from its code, e.g. func(c apierror.Code) string { return "https://docs.example.com/errors/" + string(c) }. Call it during startup. Until it is set, doc_url is null.
func SetDocURL(fn func(Code) string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	docURLFunc = fn
}

// DocURL returns the docs page for c, or "" when no doc URL builder is set.
func (c Code) DocURL() string {
	registryMu.RLock()
	fn := docURLFunc
	registryMu.RUnlock()
	if fn == nil {
		return ""
	}
	return fn(c)
}
