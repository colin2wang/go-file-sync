package syncer

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
)

// EncryptFile encrypts a file using AES-256-GCM and writes the result.
// The 12-byte random nonce is prepended to the ciphertext.
func EncryptFile(src, dst string, key []byte) error {
	plaintext, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read source for encryption: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("create cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)

	out := append(nonce, ciphertext...)
	if err := os.MkdirAll(dirName(dst), 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}
	if err := os.WriteFile(dst, out, 0644); err != nil {
		return fmt.Errorf("write encrypted file: %w", err)
	}

	return nil
}

// DecryptFile decrypts a file (nonce-prefixed ciphertext) and writes the result.
func DecryptFile(src, dst string, key []byte) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read encrypted file: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("create cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := aesgcm.NonceSize()
	if len(data) < nonceSize {
		return fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	if err := os.MkdirAll(dirName(dst), 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}
	if err := os.WriteFile(dst, plaintext, 0644); err != nil {
		return fmt.Errorf("write decrypted file: %w", err)
	}

	return nil
}

func dirName(path string) string {
	// Get directory portion of path
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == os.PathSeparator {
			return path[:i]
		}
	}
	return "."
}
