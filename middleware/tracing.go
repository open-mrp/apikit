package middleware

import (
	"net/http"

	"github.com/open-mrp/apikit/tracing"
)

// Tracing starts an OpenTelemetry span for each request, named by its route pattern, continuing any incoming trace context.
func Tracing() func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return tracing.WrapHandler(next).ServeHTTP
	}
}
