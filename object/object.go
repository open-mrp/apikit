// Package object holds the shapes every resource API shares: the object type a resource reports in its "object" field, and the standard responses built on it.
package object

import "sync"

// Type is the value of a resource's "object" field, such as "customer". It is permanent once public: version transformers, includes and the deleted stub key on it.
type Type string

var (
	registryMu sync.RWMutex
	registered = map[Type]bool{TypeList: true, TypeAsyncJob: true, TypeAsyncJobResult: true}
	order      = []Type{TypeList, TypeAsyncJob, TypeAsyncJobResult}
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

// IsValid reports whether t is a registered object type.
func (t Type) IsValid() bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registered[t]
}

// EnumValues lists every registered object type, in registration order, for schema generation.
func (t Type) EnumValues() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, len(order))
	for i, v := range order {
		out[i] = string(v)
	}
	return out
}
