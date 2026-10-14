// Package object holds the shapes every resource API shares: the object type a resource reports in its "object" field, and the standard responses built on it.
package object

import "sync"

// Type is the value of a resource's "object" field, such as "customer". It is permanent once public: version transformers, includes and the deleted stub key on it.
type Type string

// builtins are the kit's own object types, valid whether or not an app registers them.
var builtins = []Type{TypeList, TypeAsyncJob, TypeAsyncJobResult}

var (
	registryMu sync.RWMutex
	registered = map[Type]bool{}
	order      []Type
)

// Register adds an app's object types, so IsValid accepts them and EnumValues lists them for schema generation. Call it during startup. Registering a type twice is harmless.
func Register(types ...Type) {
	registryMu.Lock()
	defer registryMu.Unlock()
	for _, t := range types {
		if !registered[t] {
			registered[t] = true
			order = append(order, t)
		}
	}
}

// IsValid reports whether t is one of the kit's object types or a registered one.
func (t Type) IsValid() bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registered[t] || isBuiltin(t)
}

// EnumValues lists the registered object types in registration order, then any of the kit's own the app did not register, for schema generation.
func (t Type) EnumValues() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(order)+len(builtins))
	for _, v := range order {
		out = append(out, string(v))
	}
	for _, b := range builtins {
		if !registered[b] {
			out = append(out, string(b))
		}
	}
	return out
}

func isBuiltin(t Type) bool {
	for _, b := range builtins {
		if b == t {
			return true
		}
	}
	return false
}
