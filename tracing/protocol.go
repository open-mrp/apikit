package tracing

import "strings"

// Protocol is the OTLP transport spans and metrics are exported over.
type Protocol string

const (
	// ProtocolHTTP exports over OTLP/HTTP.
	ProtocolHTTP Protocol = "http"
	// ProtocolGRPC exports over OTLP/gRPC.
	ProtocolGRPC Protocol = "grpc"
)

// IsValid reports whether p is a supported protocol.
func (p Protocol) IsValid() bool {
	switch p {
	case ProtocolHTTP, ProtocolGRPC:
		return true
	default:
		return false
	}
}

// EnumValues lists the supported protocols.
func (p Protocol) EnumValues() []string {
	return []string{string(ProtocolHTTP), string(ProtocolGRPC)}
}

// EnvironmentProduction is the deployment environment when none is configured.
const EnvironmentProduction = "production"

// getEnv reads an environment variable, trimmed.
func getEnv(key string, getenv func(string) string) string {
	return strings.TrimSpace(getenv(key))
}
