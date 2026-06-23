package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const (
	// APIKeyPrefix is the prefix for all API keys shown to users.
	APIKeyPrefix = "ork_"
	// apiKeyRandomBytes is the entropy behind the key body (256 bits).
	apiKeyRandomBytes = 32
	// APIKeyDisplayPrefixLen is how many characters of the raw key are stored for display.
	APIKeyDisplayPrefixLen = 12
)

// GenerateAPIKey creates a new API key. The raw key is returned once; only the
// hash and a short prefix should be persisted.
func GenerateAPIKey() (raw, prefix, hash string, err error) {
	b := make([]byte, apiKeyRandomBytes)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("generate api key: %w", err)
	}
	raw = APIKeyPrefix + hex.EncodeToString(b)
	prefix = raw
	if len(prefix) > APIKeyDisplayPrefixLen {
		prefix = prefix[:APIKeyDisplayPrefixLen]
	}
	hash = HashAPIKey(raw)
	return raw, prefix, hash, nil
}

// HashAPIKey returns the SHA-256 hex digest of a raw API key.
func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
