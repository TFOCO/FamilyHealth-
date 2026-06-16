package security

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// CryptoEngine provides field-level envelope encryption using AES-256-GCM.
type CryptoEngine struct {
	key      []byte
	resolver KMSResolver
	keyName  string
}

// NewCryptoEngine creates a new instance of the encryption engine with a 32-byte key.
func NewCryptoEngine(hexKey string) (*CryptoEngine, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be exactly 32 bytes (64 hex characters), got %d bytes", len(key))
	}
	return &CryptoEngine{key: key}, nil
}

// NewCryptoEngineWithResolver creates a new instance of the encryption engine that resolves its keys dynamically.
func NewCryptoEngineWithResolver(resolver KMSResolver, keyName string) (*CryptoEngine, error) {
	if resolver == nil {
		return nil, fmt.Errorf("resolver cannot be nil")
	}
	engine := &CryptoEngine{
		resolver: resolver,
		keyName:  keyName,
	}
	// Verify we can resolve the key during initialization
	if _, err := engine.getKey(); err != nil {
		return nil, fmt.Errorf("failed to resolve encryption key during initialization: %w", err)
	}
	return engine, nil
}

// getKey retrieves the key either from the dynamic resolver or static fallback.
func (c *CryptoEngine) getKey() ([]byte, error) {
	if c.resolver != nil {
		return c.resolver.ResolveKey(context.Background(), c.keyName)
	}
	if len(c.key) == 0 {
		return nil, fmt.Errorf("no encryption key configured")
	}
	return c.key, nil
}

// Encrypt encrypts a plaintext string into a hex-encoded AES-256-GCM ciphertext.
func (c *CryptoEngine) Encrypt(plaintext string) (string, error) {
	key, err := c.getKey()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve key for encryption: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to initialize block cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to initialize GCM mode: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate random nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a hex-encoded AES-256-GCM ciphertext back to plaintext.
func (c *CryptoEngine) Decrypt(hexCiphertext string) (string, error) {
	ciphertext, err := hex.DecodeString(hexCiphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode hex ciphertext: %w", err)
	}

	key, err := c.getKey()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve key for decryption: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to initialize block cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to initialize GCM mode: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext is too short to extract nonce")
	}

	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt ciphertext: %w", err)
	}

	return string(plaintext), nil
}
