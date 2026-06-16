package security

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/fastenhealth/fasten-onprem/backend/pkg/config"
)

// VaultClient defines the interface for retrieving secrets from HashiCorp Vault.
type VaultClient interface {
	GetSecret(ctx context.Context, path string, key string) (string, error)
}

// MockVaultClient is a mock implementation of VaultClient for testing.
type MockVaultClient struct {
	secrets map[string]map[string]string
}

// NewMockVaultClient creates a new MockVaultClient.
func NewMockVaultClient() *MockVaultClient {
	return &MockVaultClient{
		secrets: make(map[string]map[string]string),
	}
}

// SetSecret sets a secret in the MockVaultClient.
func (m *MockVaultClient) SetSecret(path, key, value string) {
	if m.secrets[path] == nil {
		m.secrets[path] = make(map[string]string)
	}
	m.secrets[path][key] = value
}

// GetSecret retrieves a secret from the MockVaultClient.
func (m *MockVaultClient) GetSecret(ctx context.Context, path string, key string) (string, error) {
	if m.secrets == nil {
		return "", fmt.Errorf("vault client not initialized")
	}
	pathSecrets, exists := m.secrets[path]
	if !exists {
		return "", fmt.Errorf("vault secret path %q not found", path)
	}
	val, exists := pathSecrets[key]
	if !exists {
		return "", fmt.Errorf("key %q not found in vault path %q", key, path)
	}
	return val, nil
}

// KMSResolver resolves database encryption keys from various sources.
type KMSResolver interface {
	ResolveKey(ctx context.Context, keyName string) ([]byte, error)
}

type defaultKMSResolver struct {
	config      config.Interface
	vaultClient VaultClient
	vaultPath   string
}

// NewKMSResolver constructs a new KMSResolver instance.
func NewKMSResolver(cfg config.Interface, vaultClient VaultClient, vaultPath string) KMSResolver {
	if vaultPath == "" {
		vaultPath = "secret/data/keys"
	}
	return &defaultKMSResolver{
		config:      cfg,
		vaultClient: vaultClient,
		vaultPath:   vaultPath,
	}
}

// ResolveKey retrieves the encryption key in the following fallback order:
// 1. External KMS Provider (VaultClient)
// 2. Environmental Secrets (os.Getenv)
// 3. Local Config (config.Interface)
func (r *defaultKMSResolver) ResolveKey(ctx context.Context, keyName string) ([]byte, error) {
	// 1. Try Vault client if configured
	if r.vaultClient != nil {
		secretVal, err := r.vaultClient.GetSecret(ctx, r.vaultPath, keyName)
		if err == nil && secretVal != "" {
			decodedKey, parseErr := parseKey(secretVal)
			if parseErr == nil {
				return decodedKey, nil
			}
		}
	}

	// 2. Try Environmental Secrets
	// Derive environment variable names to look up
	// e.g., FASTEN_DATABASE_ENCRYPTION_KEY or keyName directly
	envKeys := []string{
		strings.ReplaceAll(strings.ToUpper(keyName), ".", "_"),
		strings.ReplaceAll(strings.ToUpper(keyName), "-", "_"),
		keyName,
	}
	if strings.Contains(strings.ToLower(keyName), "database") || strings.Contains(strings.ToLower(keyName), "db") {
		envKeys = append(envKeys, "DATABASE_ENCRYPTION_KEY", "DB_ENCRYPTION_KEY")
	}

	for _, envKey := range envKeys {
		val := os.Getenv(envKey)
		if val != "" {
			decodedKey, parseErr := parseKey(val)
			if parseErr == nil {
				return decodedKey, nil
			}
		}
	}

	// 3. Try Local Config if configured
	if r.config != nil {
		if r.config.IsSet(keyName) {
			val := r.config.GetString(keyName)
			if val != "" {
				decodedKey, parseErr := parseKey(val)
				if parseErr == nil {
					return decodedKey, nil
				}
			}
		}

		// Try fallback keys in local config
		fallbackKeys := []string{"database.encryption.key", "encryption.key"}
		for _, fallbackKey := range fallbackKeys {
			if r.config.IsSet(fallbackKey) {
				val := r.config.GetString(fallbackKey)
				if val != "" {
					decodedKey, parseErr := parseKey(val)
					if parseErr == nil {
						return decodedKey, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("failed to resolve encryption key for %q from any provider", keyName)
}

// parseKey helper decodes a hex key or returns a raw 32-byte key.
func parseKey(keyStr string) ([]byte, error) {
	// If hex-encoded 64-char key
	if len(keyStr) == 64 {
		decoded, err := hex.DecodeString(keyStr)
		if err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}
	// If raw 32-byte key
	if len(keyStr) == 32 {
		return []byte(keyStr), nil
	}
	return nil, fmt.Errorf("resolved key must be 32 bytes or 64 hex characters, got length %d", len(keyStr))
}
