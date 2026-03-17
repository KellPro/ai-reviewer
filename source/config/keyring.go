package config

import (
	"github.com/zalando/go-keyring"
)

const (
	// keyringService is the service name used in the OS keychain.
	keyringService = "ai-reviewer"

	// KeyAPIKey is the keyring account for the LLM API key.
	KeyAPIKey = "api-key"
	// KeyBBToken is the keyring account for the Bitbucket API token.
	KeyBBToken = "bb-token"
)

// SaveSecret stores a secret in the OS keyring.
func SaveSecret(key, value string) error {
	return keyring.Set(keyringService, key, value)
}

// GetSecret retrieves a secret from the OS keyring.
// Returns an empty string if the key is not found or an error occurs.
func GetSecret(key string) string {
	val, err := keyring.Get(keyringService, key)
	if err != nil {
		return ""
	}
	return val
}

// DeleteSecret removes a secret from the OS keyring.
func DeleteSecret(key string) error {
	return keyring.Delete(keyringService, key)
}
