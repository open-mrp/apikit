package middleware

import (
	"net/http"

	"github.com/open-mrp/apikit/apierror"
	"github.com/open-mrp/apikit/transport"
)

// IPBlock rejects requests from the given client IPs with a 403. Run it early so blocked traffic is not rate-limited, authenticated or otherwise processed. trustedProxyHops is how many proxies in front of the server append to X-Forwarded-For.
func IPBlock(blocked []string, trustedProxyHops int) func(http.HandlerFunc) http.HandlerFunc {
	set := make(map[string]struct{}, len(blocked))
	for _, ip := range blocked {
		set[ip] = struct{}{}
	}
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if ip := transport.GetClientIP(r, trustedProxyHops); ip != nil {
				if _, ok := set[ip.String()]; ok {
					transport.RespondWithAPIError(r.Context(), w, apierror.NewAuthorizationError("Access denied."))
					return
				}
			}
			next.ServeHTTP(w, r)
		}
	}
}
