package middleware

import (
	"net/http"
	"slices"

	"github.com/open-mrp/apikit/apierror"
	"github.com/open-mrp/apikit/appctx"
	"github.com/open-mrp/apikit/transport"
	"github.com/open-mrp/apikit/version"
)

// Version requires every request to name a registered API version in the version header (version.Header), and puts it on the context for the endpoint to upgrade requests from and downgrade responses to. Requests to skipPaths, such as a health check, need no version.
func Version(skipPaths ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if slices.Contains(skipPaths, r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			name := version.Header()
			requested := r.Header.Get(name)
			if requested == "" {
				transport.RespondWithAPIError(r.Context(), w, apierror.NewAPIVersionRequiredError(name, version.Latest().String()))
				return
			}
			v, err := version.Parse(requested)
			if err != nil {
				transport.RespondWithAPIError(r.Context(), w, apierror.NewAPIVersionInvalidError(name, requested, version.SupportedVersionStrings()))
				return
			}
			next.ServeHTTP(w, r.WithContext(appctx.WithAPIVersion(r.Context(), v)))
		}
	}
}
