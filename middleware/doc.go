// Package middleware holds the HTTP middleware every API shares. Each constructor returns a func(http.HandlerFunc) http.HandlerFunc for router.Router.AddMiddleware; an app adds its own, such as authentication, alongside them.
package middleware
