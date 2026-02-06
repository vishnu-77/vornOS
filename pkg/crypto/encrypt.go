package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
)

const (
	// DEKSize is the size of the data encryption key in bytes (256-bit).
	DEKSize = 32
	// NonceSize is the AES-GCM nonce size in bytes.
	NonceSize = 12
	// GCMTagSize is the AES-GCM authentication tag size.
	GCMTagSize = 16
)

// GenerateDEK creates a cryptographically random 256-bit data encryption key.
func GenerateDEK() ([]byte, error) {
	dek := make([]byte, DEKSize)
	if _, err := rand.Read(dek); err != nil {
		return nil, fmt.Errorf("generate DEK: %w", err)
	}
	return dek, nil
}

// Encrypt performs AES-256-GCM encryption.
// The aad (additional authenticated data) is integrity-protected but not encrypted.
// Returns: nonce(12) || ciphertext || tag(16).
func Encrypt(plaintext, dek, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Seal appends ciphertext+tag to nonce
	ciphertext := gcm.Seal(nonce, nonce, plaintext, aad)
	return ciphertext, nil
}

// Decrypt performs AES-256-GCM decryption.
// Input format: nonce(12) || ciphertext || tag(16).
func Decrypt(ciphertext, dek, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize+GCMTagSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := ciphertext[:nonceSize]
	ct := ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ct, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w (possible tampering or wrong key)", err)
	}
	return plaintext, nil
}

// EncryptFile encrypts src to dst using AES-256-GCM.
// The modelID is used as AAD to bind the ciphertext to a specific model.
// For large files, the entire file is read into memory (acceptable for model files
// which must fit in GPU VRAM anyway).
func EncryptFile(src, dst string, dek []byte, modelID string) error {
	plaintext, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}

	ciphertext, err := Encrypt(plaintext, dek, []byte(modelID))
	if err != nil {
		return err
	}

	if err := os.WriteFile(dst, ciphertext, 0600); err != nil {
		return fmt.Errorf("write encrypted file: %w", err)
	}
	return nil
}

// DecryptFile decrypts src to dst using AES-256-GCM.
func DecryptFile(src, dst string, dek []byte, modelID string) error {
	ciphertext, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read encrypted file: %w", err)
	}

	plaintext, err := Decrypt(ciphertext, dek, []byte(modelID))
	if err != nil {
		return err
	}

	if err := os.WriteFile(dst, plaintext, 0600); err != nil {
		return fmt.Errorf("write decrypted file: %w", err)
	}
	return nil
}

// WrapDEK encrypts a DEK with a key-encryption key (KEK) using AES-256-GCM.
// Used to protect the DEK at rest.
func WrapDEK(dek, kek []byte) ([]byte, error) {
	return Encrypt(dek, kek, []byte("vorn-dek-wrap"))
}

// UnwrapDEK decrypts a wrapped DEK using the KEK.
func UnwrapDEK(wrapped, kek []byte) ([]byte, error) {
	return Decrypt(wrapped, kek, []byte("vorn-dek-wrap"))
}

// GenerateNonce creates a random 12-byte nonce.
func GenerateNonce() ([]byte, error) {
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	return nonce, nil
}
