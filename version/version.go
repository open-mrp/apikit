// Package version models dated API versions and the transformers that let one server answer every supported version.
//
// An app registers its versions at startup with RegisterVersions; the newest becomes Latest. A request names its version in a header (Header, set with SetHeader), and responses are built in the latest shape, then downgraded by the registered Transformers to the version the caller asked for.
package version

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// APIVersion is one API version.
//
// Stable format: <minor>.<patch>.<codename> (e.g. 1.0.forge)
// Preview format: <minor>.<patch>.<codename>-preview.<n> (e.g. 1.1.forge-preview.1)
type APIVersion struct {
	Version   string // Full version string, e.g. "1.0.forge" or "1.1.forge-preview.1"
	Minor     int    // Minor version number
	Patch     int    // Patch version number
	Codename  string // Version codename, e.g. "forge"
	Preview   int    // Preview number (0 for stable releases)
	IsPreview bool   // True for a preview version

	// DeprecatedAt (optional) is when this version was deprecated; responses to it then carry a Deprecation header.
	DeprecatedAt time.Time
	// SunsetAt (optional) is when this version stops being served; responses to it then carry a Sunset header.
	SunsetAt time.Time
}

var (
	// stableRegex matches <minor>.<patch>.<codename>.
	stableRegex = regexp.MustCompile(`^([0-9]+)\.([0-9]+)\.([a-z][a-z0-9-]*)$`)
	// previewRegex matches <minor>.<patch>.<codename>-preview.<n>.
	previewRegex = regexp.MustCompile(`^([0-9]+)\.([0-9]+)\.([a-z][a-z0-9-]*)-preview\.([0-9]+)$`)
)

// New parses a version string's format, without checking that the version is registered. Use it to define an app's versions; use Parse for a version a caller sent.
func New(s string) (APIVersion, error) {
	if m := previewRegex.FindStringSubmatch(s); m != nil {
		preview, err := strconv.Atoi(m[4])
		if err != nil {
			return APIVersion{}, fmt.Errorf("invalid preview number in %s", s)
		}
		v, err := newVersion(s, m[1], m[2], m[3])
		v.Preview, v.IsPreview = preview, true
		return v, err
	}
	if m := stableRegex.FindStringSubmatch(s); m != nil {
		return newVersion(s, m[1], m[2], m[3])
	}
	return APIVersion{}, fmt.Errorf("invalid version format: %s (expected <minor>.<patch>.<codename> or <minor>.<patch>.<codename>-preview.<n>)", s)
}

func newVersion(s, minor, patch, codename string) (APIVersion, error) {
	mi, err := strconv.Atoi(minor)
	if err != nil {
		return APIVersion{}, fmt.Errorf("invalid minor version in %s", s)
	}
	pa, err := strconv.Atoi(patch)
	if err != nil {
		return APIVersion{}, fmt.Errorf("invalid patch version in %s", s)
	}
	return APIVersion{Version: s, Minor: mi, Patch: pa, Codename: codename}, nil
}

// MustNew is New for version definitions known to be well-formed; it panics otherwise.
func MustNew(s string) APIVersion {
	v, err := New(s)
	if err != nil {
		panic(err)
	}
	return v
}

var (
	registryMu sync.RWMutex
	supported  []APIVersion // newest first
	header     = "API-Version"
)

// RegisterVersions adds an app's versions. Call it once at startup with every supported version. It panics on a duplicate, since two definitions of one version would disagree about its shape.
func RegisterVersions(versions ...APIVersion) {
	registryMu.Lock()
	defer registryMu.Unlock()
	for _, v := range versions {
		if slices.ContainsFunc(supported, v.Equal) {
			panic(fmt.Sprintf("version: %s is already registered", v.Version))
		}
		supported = append(supported, v)
	}
	slices.SortFunc(supported, func(a, b APIVersion) int {
		switch {
		case a.After(b):
			return -1
		case a.Before(b):
			return 1
		default:
			return 0
		}
	})
}

// Supported returns the registered versions, newest first.
func Supported() []APIVersion {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return slices.Clone(supported)
}

// Latest returns the newest registered version. It panics when none is registered, since nothing can be served without one.
func Latest() APIVersion {
	registryMu.RLock()
	defer registryMu.RUnlock()
	if len(supported) == 0 {
		panic("version: no API versions registered")
	}
	return supported[0]
}

// SetHeader names the request header that carries the API version, e.g. "Example-Version". The default is "API-Version". Call it during startup.
func SetHeader(name string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	header = name
}

// Header returns the name of the API version header.
func Header() string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return header
}

// Parse returns the registered version a caller named. A malformed string and a well-formed but unknown version fail with different messages, so a caller can tell an unparseable header from a wrong version.
func Parse(s string) (APIVersion, error) {
	// Only the shape is checked here: a well-formed version with an out-of-range number is still just one that is not supported.
	if !previewRegex.MatchString(s) && !stableRegex.MatchString(s) {
		return APIVersion{}, fmt.Errorf("invalid version format: %s (expected <minor>.<patch>.<codename> or <minor>.<patch>.<codename>-preview.<n>)", s)
	}
	for _, v := range Supported() {
		if v.Version == s {
			return v, nil
		}
	}
	return APIVersion{}, fmt.Errorf("unsupported API version: %s", s)
}

// MustParse parses a registered version string and panics if it fails.
func MustParse(s string) APIVersion {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

// Before reports whether v is older than other. Order: minor, then patch, then codename, then preview (a preview is before its stable release).
func (v APIVersion) Before(other APIVersion) bool {
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	if v.Patch != other.Patch {
		return v.Patch < other.Patch
	}
	if c := strings.Compare(v.Codename, other.Codename); c != 0 {
		return c < 0
	}
	if v.IsPreview && other.IsPreview {
		return v.Preview < other.Preview
	}
	return v.IsPreview && !other.IsPreview
}

// After reports whether v is newer than other.
func (v APIVersion) After(other APIVersion) bool {
	return other.Before(v)
}

// Equal reports whether v and other are the same version.
func (v APIVersion) Equal(other APIVersion) bool {
	return v.Version == other.Version
}

// String returns the full version string.
func (v APIVersion) String() string {
	return v.Version
}

// SupportedVersionStrings returns every registered version string, newest first.
func SupportedVersionStrings() []string {
	vs := Supported()
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Version
	}
	return out
}

// IsSupported reports whether s names a registered version.
func IsSupported(s string) bool {
	return slices.Contains(SupportedVersionStrings(), s)
}
