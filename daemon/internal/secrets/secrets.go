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

// PrepareRotation generates a new AES-256-GCM key without writing it to disk.
// It returns:
//   - openOld: decrypts using the current active key
//   - sealNew: encrypts using the newly generated key
//   - commit: atomically writes the new key to keyPath and activates it
//
// Call commit only after all DB re-encryption has committed. On any failure,
// discard the new ciphertexts and do not call commit.
func PrepareRotation(keyPath string) (
	openOld func(string) (string, error),
	sealNew func(string) (string, error),
	commit func() error,
	err error,
) {
	newKey := make([]byte, 32)
	if _, err = rand.Read(newKey); err != nil {
		err = fmt.Errorf("generating new key: %w", err)
		return
	}
	block, cipherErr := aes.NewCipher(newKey)
	if cipherErr != nil {
		err = fmt.Errorf("creating new cipher: %w", cipherErr)
		return
	}
	newGCM, gcmErr := cipher.NewGCM(block)
	if gcmErr != nil {
		err = fmt.Errorf("creating new GCM: %w", gcmErr)
		return
	}

	openOld = Open

	sealNew = func(plaintext string) (string, error) {
		if plaintext == "" {
			return "", nil
		}
		nonce := make([]byte, newGCM.NonceSize())
		if _, readErr := io.ReadFull(rand.Reader, nonce); readErr != nil {
			return "", fmt.Errorf("generating nonce: %w", readErr)
		}
		sealed := newGCM.Seal(nonce, nonce, []byte(plaintext), nil)
		return base64.StdEncoding.EncodeToString(sealed), nil
	}

	commit = func() error {
		tmp := keyPath + ".new"
		if writeErr := os.WriteFile(tmp, newKey, 0600); writeErr != nil {
			return fmt.Errorf("writing new key: %w", writeErr)
		}
		if renameErr := os.Rename(tmp, keyPath); renameErr != nil {
			os.Remove(tmp)
			return fmt.Errorf("installing new key: %w", renameErr)
		}
		mu.Lock()
		gcm = newGCM
		ok = true
		mu.Unlock()
		return nil
	}

	return
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
