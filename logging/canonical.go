// Package logging builds canonical log lines: one structured slog record per request, carrying everything needed to find and explain it later (who called, what it was, how long it took, how it ended). Transports (the HTTP middleware, and later gRPC interceptors) add their own fields to the shared ones built here.
package logging

import (
	"context"
	"log/slog"
	"reflect"

	"go.opentelemetry.io/otel/trace"

	"github.com/open-mrp/apikit/appctx"
)

// TypeCanonicalLogLine is the value of the "type" attribute on every canonical log line, for filtering them in a log backend.
const TypeCanonicalLogLine = "canonical-log-line"

// IdentityAttrer is implemented by an app's identity type to say what a log line records about the caller, such as the actor type and ID. Keep the attributes low in volume and free of secrets.
type IdentityAttrer interface {
	LogAttrs() []slog.Attr
}

// RequestAttrs returns the attributes every canonical log line carries from ctx: the request ID, the caller identity's attributes (when its type implements IdentityAttrer), and the trace and span IDs of a recording span. Absent values are omitted.
func RequestAttrs(ctx context.Context) []slog.Attr {
	var attrs []slog.Attr

	if requestID, ok := appctx.GetRequestID(ctx); ok {
		attrs = append(attrs, slog.String("request_id", requestID))
	}

	if identity, ok := appctx.AnyIdentity(ctx); ok {
		if attrer, ok := identity.(IdentityAttrer); ok && !isNilPointer(attrer) {
			attrs = append(attrs, attrer.LogAttrs()...)
		}
	}

	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		if spanCtx := span.SpanContext(); spanCtx.IsValid() {
			attrs = append(attrs,
				slog.String("trace_id", spanCtx.TraceID().String()),
				slog.String("span_id", spanCtx.SpanID().String()),
			)
		}
	}

	return attrs
}

// isNilPointer reports a typed nil stored as the identity: it satisfies the interface, but calling LogAttrs on it would usually panic.
func isNilPointer(v any) bool {
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Pointer && rv.IsNil()
}
