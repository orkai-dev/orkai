package testsupport

import "github.com/orkai-dev/orkai/apps/api/internal/providers"

// NewProviders returns a providers.Registry wired with the built-in providers,
// using the given FakeStore's settings for provider configuration. Use in
// service/handler tests that construct services depending on the registry.
func NewProviders(fs *FakeStore) *providers.Registry {
	return providers.New(fs.Settings(), NewTestLogger())
}
