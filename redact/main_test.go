package redact_test

import (
	"os"
	"strings"
	"testing"

	"github.com/open-mrp/apikit/redact"
)

// Registrations are process-wide and made before any test runs, the same way an app makes them at startup.
func TestMain(m *testing.M) {
	redact.RegisterSensitiveTag("internal")
	redact.RegisterKeyedMapTag("internal_keys", func(key string) bool {
		return strings.HasPrefix(key, "internal_")
	})
	os.Exit(m.Run())
}
