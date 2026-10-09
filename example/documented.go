package example

// Documented is implemented by a request or response type that supplies its own representative value, which the OpenAPI generator uses as the schema's example in place of one built from zero values.
type Documented interface {
	SchemaExample() any
}
