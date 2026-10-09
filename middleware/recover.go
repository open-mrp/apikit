package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/open-mrp/apikit/apierror"
	"github.com/open-mrp/apikit/appctx"
	"github.com/open-mrp/apikit/transport"
)

// Recover turns a handler panic into a 500, logs the panic with its stack, and records it on the request log, instead of letting the connection drop.
func Recover() func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					// The client only sees "Something went wrong", so without this the panic and its stack would be lost.
					slog.ErrorContext(r.Context(), "panic recovered in HTTP handler",
						"panic", fmt.Sprintf("%v", rec),
						"method", r.Method,
						"path", r.URL.Path,
						"stack", string(debug.Stack()),
					)
					if rl, ok := appctx.GetRequestLog(r.Context()); ok && (rl.ErrorMessage == nil || *rl.ErrorMessage == "") {
						msg := "An unexpected error occurred during the request"
						rl.ErrorMessage = &msg
					}
					transport.RespondWithAPIError(r.Context(), w, apierror.NewInternalError(fmt.Errorf("%v", rec), fmt.Sprintf("A panic occurred during the request: %v", rec)))
				}
			}()
			next.ServeHTTP(w, r)
		}
	}
}
