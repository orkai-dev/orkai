package secret

import (
	"encoding/json"
	"fmt"
)

// EncryptJSONFieldValues encrypts string values for keys where isSecret returns true.
func EncryptJSONFieldValues(s Store, raw json.RawMessage, isSecret func(string) bool) (json.RawMessage, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return raw, err
	}
	changed := false
	for k, v := range cfg {
		if !isSecret(k) {
			continue
		}
		str, ok := v.(string)
		if !ok || str == "" {
			continue
		}
		enc, err := s.Encrypt(str)
		if err != nil {
			return nil, fmt.Errorf("encrypt config field %q: %w", k, err)
		}
		cfg[k] = enc
		changed = true
	}
	if !changed {
		return raw, nil
	}
	return json.Marshal(cfg)
}

// DecryptJSONFieldValues decrypts string values for keys where isSecret returns true.
func DecryptJSONFieldValues(s Store, raw json.RawMessage, isSecret func(string) bool) (json.RawMessage, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return raw, err
	}
	changed := false
	for k, v := range cfg {
		if !isSecret(k) {
			continue
		}
		str, ok := v.(string)
		if !ok || str == "" {
			continue
		}
		plain, err := s.Decrypt(str)
		if err != nil {
			return nil, fmt.Errorf("decrypt config field %q: %w", k, err)
		}
		cfg[k] = plain
		changed = true
	}
	if !changed {
		return raw, nil
	}
	return json.Marshal(cfg)
}

// EncryptStringMap encrypts every value in the map.
func EncryptStringMap(s Store, m map[string]string) (map[string]string, error) {
	if len(m) == 0 {
		return m, nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if v == "" {
			out[k] = v
			continue
		}
		enc, err := s.Encrypt(v)
		if err != nil {
			return nil, fmt.Errorf("encrypt secret %q: %w", k, err)
		}
		out[k] = enc
	}
	return out, nil
}

// DecryptStringMap decrypts every value in the map.
func DecryptStringMap(s Store, m map[string]string) (map[string]string, error) {
	if len(m) == 0 {
		return m, nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if v == "" {
			out[k] = v
			continue
		}
		plain, err := s.Decrypt(v)
		if err != nil {
			return nil, fmt.Errorf("decrypt secret %q: %w", k, err)
		}
		out[k] = plain
	}
	return out, nil
}
