package secret

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	s := NewFromSetupSecret("test-setup-secret-at-least-32-chars!!")
	enc, err := s.Encrypt("super-secret-token")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(enc, wirePrefix))

	plain, err := s.Decrypt(enc)
	require.NoError(t, err)
	assert.Equal(t, "super-secret-token", plain)
}

func TestEncryptEmpty(t *testing.T) {
	s := NewFromSetupSecret("test-setup-secret-at-least-32-chars!!")
	enc, err := s.Encrypt("")
	require.NoError(t, err)
	assert.Empty(t, enc)
}

func TestDecryptNotEncrypted(t *testing.T) {
	s := NewFromSetupSecret("test-setup-secret-at-least-32-chars!!")
	_, err := s.Decrypt("plaintext")
	assert.ErrorIs(t, err, ErrNotEncrypted)
}

func TestDecryptWrongKey(t *testing.T) {
	s1 := NewFromSetupSecret("test-setup-secret-at-least-32-chars!!")
	s2 := NewFromSetupSecret("different-setup-secret-32-chars!!")
	enc, err := s1.Encrypt("secret")
	require.NoError(t, err)
	_, err = s2.Decrypt(enc)
	assert.ErrorIs(t, err, ErrDecrypt)
}

func TestDecryptTampered(t *testing.T) {
	s := NewFromSetupSecret("test-setup-secret-at-least-32-chars!!")
	enc, err := s.Encrypt("secret")
	require.NoError(t, err)
	tampered := enc[:len(enc)-1] + "X"
	_, err = s.Decrypt(tampered)
	assert.ErrorIs(t, err, ErrDecrypt)
}

func TestEncryptJSONFieldValues(t *testing.T) {
	s := NewFromSetupSecret("test-setup-secret-at-least-32-chars!!")
	raw := []byte(`{"token":"abc","name":"my-repo"}`)
	enc, err := EncryptJSONFieldValues(s, raw, IsResourceConfigKey)
	require.NoError(t, err)
	assert.NotEqual(t, string(raw), string(enc))
	assert.Contains(t, string(enc), wirePrefix)

	dec, err := DecryptJSONFieldValues(s, enc, IsResourceConfigKey)
	require.NoError(t, err)
	assert.JSONEq(t, string(raw), string(dec))
}

func TestEncryptStringMap(t *testing.T) {
	s := NewFromSetupSecret("test-setup-secret-at-least-32-chars!!")
	enc, err := EncryptStringMap(s, map[string]string{"API_KEY": "xyz"})
	require.NoError(t, err)
	assert.Contains(t, enc["API_KEY"], wirePrefix)

	plain, err := DecryptStringMap(s, enc)
	require.NoError(t, err)
	assert.Equal(t, "xyz", plain["API_KEY"])
}
