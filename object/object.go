// Package object holds the shapes every resource API shares: the object type a resource reports in its "object" field, and the standard responses built on it.
package object

// Type is the value of a resource's "object" field, such as "customer". It is permanent once public: version transformers, includes and the deleted stub key on it.
type Type string
