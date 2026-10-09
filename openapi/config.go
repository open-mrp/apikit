// Package openapi generates an OpenAPI 3 spec, Stainless SDK configs and an agent-tool catalog from an app's endpoint groups. The endpoint definitions are the source of truth: routes, request and response types, docstrings (read from the Go doc comments on the endpoint and resource types), examples and access policies all come from them, so the docs cannot drift from the code.
package openapi

import (
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/open-mrp/apikit/endpoint"
	"github.com/open-mrp/apikit/version"
)

// Config describes the spec to generate.
type Config struct {
	// Groups (required) are the app's endpoint groups.
	Groups []endpoint.APIEndpointGroup
	// Title and Description (required) name the API in the spec's info block.
	Title, Description string
	// Version (optional; default: version.Latest()) is the spec's info.version.
	Version string
	// PublicOnly (optional) leaves out endpoints that are not Public, for the published reference.
	PublicOnly bool
	// Servers (optional) are the spec's servers.
	Servers []Server
	// SecuritySchemes (optional) are the spec's security schemes, e.g. a bearer API key.
	SecuritySchemes map[string]SecuritySchemeSpec
	// Security (optional) is the spec's default security requirement. Include an empty requirement to allow unauthenticated or cookie access.
	Security []map[string][]string
	// Transforms (optional) are applied to the finished document, for edits the generator cannot infer.
	Transforms []Transform
	// DescribeAuth (optional) turns an endpoint's Auth policy into a sentence appended to its description, e.g. "This endpoint requires the permission: `orders:read`.". Return "" for none.
	DescribeAuth func(policy any) string
	// ParamDescriptions (optional) describe header and cookie parameters a request struct does not document, by name.
	ParamDescriptions map[string]string
	// Examples (optional) supply sample values where the types do not.
	Examples Examples
	// NamedEnums (optional) are string enum types emitted as one named component schema instead of inline at every use.
	NamedEnums []reflect.Type
}

// Examples supplies sample values for parameters and list cursors, from the app's sample data. Each hook returns "" or nil to fall back to the generator's own.
type Examples struct {
	// PathParam returns a sample for a path parameter, given its name, the route template and the request struct field it binds to.
	PathParam func(param, route, field string) string
	// QueryParam returns a sample for a query parameter, by name without a trailing "[]".
	QueryParam func(param string) any
	// ListCursor returns the documented cursor for the page after a list whose last item, of Go type itemType, serializes to item. The default signs a string cursor from the item's id and created_at or occurred_at.
	ListCursor func(itemType string, item map[string]any) string
}

// activeConfig is the Config of the generation in progress, read from deep inside schema building. Build, WriteStainlessConfig and AgentTools hold genMu for their whole run, so concurrent generations take turns; the pointer is atomic so code that reads it outside a run sees a whole Config.
var (
	genMu        sync.Mutex
	activeConfig atomic.Pointer[Config]
)

// active returns the Config of the generation in progress, or the zero Config.
func active() *Config {
	if c := activeConfig.Load(); c != nil {
		return c
	}
	return &Config{}
}

// setActive makes cfg, with defaults applied, the Config of the generation in progress.
func setActive(cfg Config) *Config {
	c := cfg.withDefaults()
	activeConfig.Store(&c)
	return &c
}

func (c Config) withDefaults() Config {
	if c.Version == "" {
		c.Version = version.Latest().String()
	}
	return c
}
