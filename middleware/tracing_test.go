package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/open-mrp/apikit/appctx"
)

// The span starts inside the router, after the request log, so Tracing is what links the two.
func TestTracing_LinksTheRequestLog(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)))
	t.Cleanup(func() { otel.SetTracerProvider(previous) })

	rl := &appctx.RequestLog{ID: "rq_traced"}
	h := Tracing()(func(http.ResponseWriter, *http.Request) {})
	r := httptest.NewRequest(http.MethodGet, "/v1/things", nil)
	h(httptest.NewRecorder(), r.WithContext(appctx.WithRequestLog(r.Context(), rl)))

	ended := spans.Ended()
	if len(ended) != 1 {
		t.Fatalf("%d spans", len(ended))
	}
	if rl.TraceID == nil || *rl.TraceID != ended[0].SpanContext().TraceID().String() {
		t.Errorf("request log trace ID %v, span %s", rl.TraceID, ended[0].SpanContext().TraceID())
	}
	found := false
	for _, a := range ended[0].Attributes() {
		if a.Key == "request.id" && a.Value.AsString() == "rq_traced" {
			found = true
		}
	}
	if !found {
		t.Error("span has no request.id")
	}
}
