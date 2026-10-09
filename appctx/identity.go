package appctx

import "context"

const identityKey contextKey = "identity"

// WithIdentity returns a child context carrying the caller's identity. Each app defines its own identity type; the kit stores it without knowing its shape.
func WithIdentity[T any](ctx context.Context, identity T) context.Context {
	return context.WithValue(ctx, identityKey, identity)
}

// Identity returns the identity ctx carries, if one is present and is a T. Like any typed getter, ok only reports that a T is stored: a stored nil pointer comes back as (nil, true).
func Identity[T any](ctx context.Context) (T, bool) {
	identity, ok := ctx.Value(identityKey).(T)
	return identity, ok
}

// AnyIdentity returns the identity ctx carries whatever its type, for kit code such as logging that only needs an interface the app's identity implements.
func AnyIdentity(ctx context.Context) (any, bool) {
	identity := ctx.Value(identityKey)
	return identity, identity != nil
}
