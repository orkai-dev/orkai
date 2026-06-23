package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/hkdf"
)

const (
	wirePrefix = "enc:v1:"
	hkdfInfo   = "orkai-secrets-v1"
)

var (
	ErrNotEncrypted = errors.New("value is not encrypted")
	ErrDecrypt      = errors.New("decryption failed")
)

// Store encrypts and decrypts credential strings at rest.
type Store interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

type aesStore struct {
	key []byte
}

// NewFromSetupSecret derives an AES-256 key from SETUP_SECRET via HKDF-SHA256.
func NewFromSetupSecret(setupSecret string) Store {
	key := make([]byte, 32)
	r := hkdf.New(sha256.New, []byte(setupSecret), nil, []byte(hkdfInfo))
	_, _ = r.Read(key)
	return &aesStore{key: key}
}

func (s *aesStore) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return wirePrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *aesStore) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if !strings.HasPrefix(ciphertext, wirePrefix) {
		return "", ErrNotEncrypted
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(ciphertext, wirePrefix))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecrypt, err)
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", ErrDecrypt
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecrypt, err)
	}
	return string(plaintext), nil
}

// EncryptOptional encrypts non-empty plaintext; empty string stays empty.
func EncryptOptional(s Store, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	return s.Encrypt(plaintext)
}

// DecryptOptional decrypts non-empty ciphertext; empty string stays empty.
func DecryptOptional(s Store, ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	return s.Decrypt(ciphertext)
}
