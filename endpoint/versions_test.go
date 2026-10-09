package endpoint

import "github.com/open-mrp/apikit/version"

// The version an app would register; the tests treat it as latest.
func init() {
	version.RegisterVersions(version.MustNew("1.0.forge-preview.8"))
}
