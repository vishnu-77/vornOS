package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// HashFile computes the SHA-256 hash of a file, streaming to handle large models.
// Returns the hex-encoded hash string.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Sign computes the SHA-256 hash of data and signs it with an Ed25519 private key.
// Returns the 64-byte signature and the hex-encoded hash.
func Sign(data []byte, key ed25519.PrivateKey) (signature []byte, hash string, err error) {
	if len(key) != ed25519.PrivateKeySize {
		return nil, "", fmt.Errorf("invalid Ed25519 private key size: %d", len(key))
	}

	digest := sha256.Sum256(data)
	hash = hex.EncodeToString(digest[:])
	signature = ed25519.Sign(key, digest[:])
	return signature, hash, nil
}

// SignFile computes SHA-256 of a file and signs the hash with an Ed25519 private key.
// Returns the signature and hex hash.
func SignFile(path string, key ed25519.PrivateKey) (signature []byte, hash string, err error) {
	hash, err = HashFile(path)
	if err != nil {
		return nil, "", err
	}

	digest, err := hex.DecodeString(hash)
	if err != nil {
		return nil, "", fmt.Errorf("decode hash: %w", err)
	}

	signature = ed25519.Sign(key, digest)
	return signature, hash, nil
}

// Verify checks an Ed25519 signature against a set of trusted public keys.
// It tries each key in order and returns the index of the key that verified,
// or an error if none did. The hash must be hex-encoded SHA-256.
func Verify(hash string, signature []byte, keys []ed25519.PublicKey) (keyIndex int, err error) {
	if len(keys) == 0 {
		return -1, fmt.Errorf("no public keys provided")
	}
	if len(signature) != ed25519.SignatureSize {
		return -1, fmt.Errorf("invalid signature size: %d (expected %d)", len(signature), ed25519.SignatureSize)
	}

	digest, err := hex.DecodeString(hash)
	if err != nil {
		return -1, fmt.Errorf("invalid hash encoding: %w", err)
	}
	if len(digest) != sha256.Size {
		return -1, fmt.Errorf("invalid hash length: %d (expected %d)", len(digest), sha256.Size)
	}

	for i, key := range keys {
		if ed25519.Verify(key, digest, signature) {
			return i, nil
		}
	}
	return -1, fmt.Errorf("signature verification failed: no trusted key matched")
}
