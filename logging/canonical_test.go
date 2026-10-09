package logging

import (
	"context"
	"log/slog"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/open-mrp/apikit/appctx"
)

type testIdentity struct {
	ActorType string
	ActorID   string
}

func (i *testIdentity) LogAttrs() []slog.Attr {
	return []slog.Attr{slog.String("actor_type", i.ActorType), slog.String("actor_id", i.ActorID)}
}

type silentIdentity struct{}

func attrsToMap(attrs []slog.Attr) map[string]string {
	m := make(map[string]string, len(attrs))
	for _, a := range attrs {
		m[a.Key] = a.Value.String()
	}
	return m
}

func TestRequestAttrs_EmptyContext(t *testing.T) {
	t.Parallel()
	if attrs := RequestAttrs(context.Background()); len(attrs) != 0 {
		t.Errorf("got %v, want none", attrs)
	}
}

func TestRequestAttrs_RequestIDAndIdentity(t *testing.T) {
	t.Parallel()
	ctx := appctx.WithRequestID(context.Background(), "req_1")
	ctx = appctx.WithIdentity(ctx, &testIdentity{ActorType: "user", ActorID: "us_1"})

	m := attrsToMap(RequestAttrs(ctx))
	if m["request_id"] != "req_1" || m["actor_type"] != "user" || m["actor_id"] != "us_1" {
		t.Errorf("got %v", m)
	}
}

func TestRequestAttrs_IdentityWithoutAttrsIsSkipped(t *testing.T) {
	t.Parallel()
	ctx := appctx.WithIdentity(context.Background(), silentIdentity{})
	if attrs := RequestAttrs(ctx); len(attrs) != 0 {
		t.Errorf("got %v, want none", attrs)
	}
}

func TestRequestAttrs_NilIdentityPointerIsSkipped(t *testing.T) {
	t.Parallel()
	ctx := appctx.WithIdentity[*testIdentity](context.Background(), nil)
	if attrs := RequestAttrs(ctx); len(attrs) != 0 {
		t.Errorf("got %v, want none", attrs)
	}
}

func TestRequestAttrs_TraceIDsFromRecordingSpan(t *testing.T) {
	t.Parallel()
	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	m := attrsToMap(RequestAttrs(ctx))
	if m["trace_id"] != span.SpanContext().TraceID().String() || m["span_id"] != span.SpanContext().SpanID().String() {
		t.Errorf("got %v", m)
	}
}
