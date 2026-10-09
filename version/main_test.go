package version

import (
	"os"
	"testing"

	"github.com/open-mrp/apikit/object"
)

// Test fixtures: the versions and object types an app would define.
var (
	V1_0_Forge_Preview1 = MustNew("1.0.forge-preview.1")
	V1_0_Forge_Preview2 = MustNew("1.0.forge-preview.2")
	V1_0_Forge_Preview3 = MustNew("1.0.forge-preview.3")
	V1_0_Forge_Preview4 = MustNew("1.0.forge-preview.4")
	V1_0_Forge_Preview5 = MustNew("1.0.forge-preview.5")
	V1_0_Forge_Preview6 = MustNew("1.0.forge-preview.6")
	V1_0_Forge_Preview7 = MustNew("1.0.forge-preview.7")
	V1_0_Forge_Preview8 = MustNew("1.0.forge-preview.8")
)

const (
	testObjectUser       object.Type = "user"
	testObjectAccount    object.Type = "account"
	testObjectRequestLog object.Type = "request_log"
)

// Versions are registered process-wide before any test runs, the same way an app registers them at startup. Registered out of order to show RegisterVersions sorts them.
func TestMain(m *testing.M) {
	RegisterVersions(
		V1_0_Forge_Preview3, V1_0_Forge_Preview1, V1_0_Forge_Preview8, V1_0_Forge_Preview2,
		V1_0_Forge_Preview5, V1_0_Forge_Preview4, V1_0_Forge_Preview7, V1_0_Forge_Preview6,
	)
	os.Exit(m.Run())
}
