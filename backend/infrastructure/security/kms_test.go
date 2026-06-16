package security

import (
	"context"
	"encoding/hex"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConfig implements config.Interface for testing purposes.
type mockConfig struct {
	settings map[string]interface{}
}

func (m *mockConfig) Init() error { return nil }
func (m *mockConfig) ReadConfig(configFilePath string) error { return nil }
func (m *mockConfig) Set(key string, value interface{}) {
	if m.settings == nil {
		m.settings = make(map[string]interface{})
	}
	m.settings[key] = value
}
func (m *mockConfig) SetDefault(key string, value interface{}) { m.Set(key, value) }
func (m *mockConfig) MergeConfigMap(cfg map[string]interface{}) error { return nil }
func (m *mockConfig) AllSettings() map[string]interface{} { return m.settings }
func (m *mockConfig) IsSet(key string) bool {
	_, exists := m.settings[key]
	return exists
}
func (m *mockConfig) Get(key string) interface{} { return m.settings[key] }
func (m *mockConfig) GetBool(key string) bool { return false }
func (m *mockConfig) GetInt(key string) int { return 0 }
func (m *mockConfig) GetString(key string) string {
	if val, ok := m.settings[key].(string); ok {
		return val
	}
	return ""
}
func (m *mockConfig) GetStringSlice(key string) []string { return nil }
func (m *mockConfig) UnmarshalKey(key string, rawVal interface{}, decoder ...viper.DecoderConfigOption) error {
	return nil
}

func TestKMSResolver_VaultSuccess(t *testing.T) {
	ctx := context.Background()
	vault := NewMockVaultClient()

	hexKey := "1111111111111111111111111111111111111111111111111111111111111111"
	vault.SetSecret("secret/data/keys", "database.key", hexKey)

	resolver := NewKMSResolver(nil, vault, "")
	key, err := resolver.ResolveKey(ctx, "database.key")
	require.NoError(t, err)

	expected, _ := hex.DecodeString(hexKey)
	assert.Equal(t, expected, key)
}

func TestKMSResolver_EnvFallback(t *testing.T) {
	ctx := context.Background()

	// Env variable format: replacing dots with underscores
	hexKey := "2222222222222222222222222222222222222222222222222222222222222222"
	t.Setenv("DATABASE_KEY", hexKey)

	resolver := NewKMSResolver(nil, nil, "")
	key, err := resolver.ResolveKey(ctx, "database.key")
	require.NoError(t, err)

	expected, _ := hex.DecodeString(hexKey)
	assert.Equal(t, expected, key)
}

func TestKMSResolver_ConfigFallback(t *testing.T) {
	ctx := context.Background()
	cfg := &mockConfig{settings: make(map[string]interface{})}

	hexKey := "3333333333333333333333333333333333333333333333333333333333333333"
	cfg.Set("database.key", hexKey)

	resolver := NewKMSResolver(cfg, nil, "")
	key, err := resolver.ResolveKey(ctx, "database.key")
	require.NoError(t, err)

	expected, _ := hex.DecodeString(hexKey)
	assert.Equal(t, expected, key)
}

func TestKMSResolver_FallbackOrder(t *testing.T) {
	ctx := context.Background()
	vault := NewMockVaultClient()
	cfg := &mockConfig{settings: make(map[string]interface{})}

	keyVault := "1111111111111111111111111111111111111111111111111111111111111111"
	keyEnv := "2222222222222222222222222222222222222222222222222222222222222222"
	keyConfig := "3333333333333333333333333333333333333333333333333333333333333333"

	vault.SetSecret("secret/data/keys", "database.key", keyVault)
	t.Setenv("DATABASE_KEY", keyEnv)
	cfg.Set("database.key", keyConfig)

	// Case 1: All are set -> Vault should win
	resolver := NewKMSResolver(cfg, vault, "")
	key, err := resolver.ResolveKey(ctx, "database.key")
	require.NoError(t, err)
	expectedVault, _ := hex.DecodeString(keyVault)
	assert.Equal(t, expectedVault, key)

	// Case 2: Vault empty/missing -> Env should win
	resolverNoVault := NewKMSResolver(cfg, nil, "")
	key, err = resolverNoVault.ResolveKey(ctx, "database.key")
	require.NoError(t, err)
	expectedEnv, _ := hex.DecodeString(keyEnv)
	assert.Equal(t, expectedEnv, key)

	// Case 3: Vault missing, Env missing -> Config should win
	os.Unsetenv("DATABASE_KEY") // clean up env for this check
	resolverNoEnv := NewKMSResolver(cfg, nil, "")
	key, err = resolverNoEnv.ResolveKey(ctx, "database.key")
	require.NoError(t, err)
	expectedConfig, _ := hex.DecodeString(keyConfig)
	assert.Equal(t, expectedConfig, key)
}

func TestKMSResolver_Failure(t *testing.T) {
	ctx := context.Background()
	resolver := NewKMSResolver(nil, nil, "")
	_, err := resolver.ResolveKey(ctx, "nonexistent.key")
	assert.Error(t, err)
}

func TestCryptoEngine_DynamicResolution(t *testing.T) {
	vault := NewMockVaultClient()
	key1 := "1111111111111111111111111111111111111111111111111111111111111111"
	vault.SetSecret("secret/data/keys", "database.key", key1)

	resolver := NewKMSResolver(nil, vault, "")
	crypto, err := NewCryptoEngineWithResolver(resolver, "database.key")
	require.NoError(t, err)

	plaintext := "sensitive health record"
	ciphertext, err := crypto.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := crypto.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)

	// Rotate key in Vault
	key2 := "9999999999999999999999999999999999999999999999999999999999999999"
	vault.SetSecret("secret/data/keys", "database.key", key2)

	// Trying to decrypt original ciphertext with the new rotated key should fail authentication
	_, err = crypto.Decrypt(ciphertext)
	assert.Error(t, err)

	// Encrypting new data should work with the rotated key
	ciphertext2, err := crypto.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted2, err := crypto.Decrypt(ciphertext2)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted2)
}
