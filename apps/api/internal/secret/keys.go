package secret

// IsResourceConfigKey reports whether a shared-resource config field holds a credential.
func IsResourceConfigKey(key string) bool {
	switch key {
	case "token", "refresh_token", "password", "secret_key", "secret_access_key",
		"client_secret", "webhook_secret", "private_key", "passphrase",
		"api_token", "api_key":
		return true
	default:
		return false
	}
}

// IsNotificationConfigKey reports whether a notification channel config field holds a credential.
func IsNotificationConfigKey(key string) bool {
	switch key {
	case "bot_token", "webhook_url":
		return true
	default:
		return false
	}
}
