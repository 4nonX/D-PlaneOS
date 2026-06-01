package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

var (
	mu  sync.RWMutex
	gcm cipher.AEAD
	ok  bool
)

// Init loads the 32-byte AES-256 key from keyPath, creating it if absent.
// Must be called once at daemon startup before any Seal or Open call.
func Init(keyPath string) error {
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(keyPath)
	if err == nil {
		if len(data) != 32 {
			return fmt.Errorf("secrets key at %s has wrong length %d (want 32)", keyPath, len(data))
		}
	} else if os.IsNotExist(err) {
		data = make([]byte, 32)
		if _, err := rand.Read(data); err != nil {
			return fmt.Errorf("generating secrets key: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
			return fmt.Errorf("creating secrets key dir: %w", err)
		}
		if err := os.WriteFile(keyPath, data, 0600); err != nil {
			return fmt.Errorf("writing secrets key: %w", err)
		}
	} else {
		return fmt.Errorf("reading secrets key: %w", err)
	}

	block, err := aes.NewCipher(data)
	if err != nil {
		return fmt.Errorf("creating AES cipher: %w", err)
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("creating GCM: %w", err)
	}
	gcm = g
	ok = true
	return nil
}

// Seal encrypts plaintext with AES-256-GCM and returns a base64-encoded blob.
// Empty input returns empty string (passthrough for optional fields).
func Seal(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	mu.RLock()
	defer mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("secrets.Init not called")
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Open decrypts a value produced by Seal.
// Empty input returns empty string.
func Open(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	mu.RLock()
	defer mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("secrets.Init not called")
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decoding ciphertext: %w", err)
	}
	ns := gcm.NonceSize()
	if len(data) < ns {
		return "", fmt.Errorf("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, data[:ns], data[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypting: %w", err)
	}
	return string(plaintext), nil
}
