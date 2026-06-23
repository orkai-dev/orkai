package cloudflare

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCredentialsValidate(t *testing.T) {
	t.Run("api token required", func(t *testing.T) {
		err := Credentials{AuthMode: AuthAPIToken}.Validate()
		require.ErrorContains(t, err, "API token is required")
	})

	t.Run("api token ok", func(t *testing.T) {
		err := Credentials{AuthMode: AuthAPIToken, APIToken: "token"}.Validate()
		require.NoError(t, err)
	})

	t.Run("api key requires email", func(t *testing.T) {
		err := Credentials{AuthMode: AuthAPIKey, APIKey: "key"}.Validate()
		require.ErrorContains(t, err, "API key and email are required")
	})

	t.Run("api key ok", func(t *testing.T) {
		err := Credentials{AuthMode: AuthAPIKey, APIKey: "key", Email: "a@b.com"}.Validate()
		require.NoError(t, err)
	})
}
