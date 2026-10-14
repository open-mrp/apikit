package middleware

import (
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/open-mrp/apikit/appctx"
	"github.com/open-mrp/apikit/tracing"
)

// Tracing starts an OpenTelemetry span for each request, named by its route pattern, continuing any incoming trace context. It runs inside the router, after RequestLog started the request's log, so it records the trace ID on that log and the request ID on the span.
func Tracing() func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return tracing.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			linkRequestLog(r)
			next(w, r)
		})).ServeHTTP
	}
}

// linkRequestLog ties the request's span and request log to each other, when both exist.
func linkRequestLog(r *http.Request) {
	rl, ok := appctx.GetRequestLog(r.Context())
	if !ok || rl == nil {
		return
	}
	span := trace.SpanFromContext(r.Context())
	sc := span.SpanContext()
	if !sc.IsValid() {
		return
	}
	span.SetAttributes(attribute.String("request.id", rl.ID))
	if rl.TraceID == nil {
		traceID := sc.TraceID().String()
		rl.TraceID = &traceID
	}
}
