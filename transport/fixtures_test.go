package transport

// Enum fixtures standing in for an app's constants: a string type with IsValid and EnumValues.
type testMode string

const (
	testModeProduction testMode = "production"
	testModeSandbox    testMode = "sandbox"
)

func (m testMode) IsValid() bool { return m == testModeProduction || m == testModeSandbox }

func (testMode) EnumValues() []string { return []string{string(testModeProduction), string(testModeSandbox)} }

type testKeyStatus string

const (
	testKeyStatusActive  testKeyStatus = "active"
	testKeyStatusRevoked testKeyStatus = "revoked"
)

func (s testKeyStatus) IsValid() bool { return s == testKeyStatusActive || s == testKeyStatusRevoked }

func (testKeyStatus) EnumValues() []string {
	return []string{string(testKeyStatusActive), string(testKeyStatusRevoked)}
}

// Bearer tokens shaped like API keys, for auth header parsing.
const (
	testAPIKeySandbox    = "testtoken_sandbox_0000000000"
	testAPIKeyProduction = "testtoken_production_000000"
)
