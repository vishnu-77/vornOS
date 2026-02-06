// Package models implements the signed, encrypted model store.
// Models are signed with Ed25519 keys and verified before any job can use them.
package models

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	vorncrypto "github.com/vornos/buildos/pkg/crypto"
)

// Artifact represents a model in the secure store.
type Artifact struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Version    string    `json:"version"`
	Format     string    `json:"format"`       // e.g. "onnx", "safetensors", "torchscript"
	SHA256     string    `json:"sha256"`        // hash of the raw model file
	Signature  []byte    `json:"signature"`     // Ed25519 signature over SHA256
	SizeBytes  int64     `json:"size_bytes"`
	Encrypted  bool      `json:"encrypted"`     // whether the on-disk blob is AES-GCM encrypted
	ImportedAt time.Time `json:"imported_at"`
	Verified   bool      `json:"verified"`      // last verification result
}

// Store manages model artifacts on disk.
type Store struct {
	mu        sync.RWMutex
	baseDir   string
	trustKeys []ed25519.PublicKey
	dek       []byte // data encryption key (nil = encryption disabled)
	artifacts map[string]*Artifact
}

// NewStore creates a model store backed by baseDir.
// trustKeys are the Ed25519 public keys accepted for verification.
// dek is the data encryption key (pass nil to disable encryption).
func NewStore(baseDir string, trustKeys []ed25519.PublicKey, dek []byte) *Store {
	s := &Store{
		baseDir:   baseDir,
		trustKeys: trustKeys,
		dek:       dek,
		artifacts: make(map[string]*Artifact),
	}
	s.loadIndex()
	return s
}

// Add imports a model from filePath, signs it, optionally encrypts it, and stores it.
func (s *Store) Add(name, version, format, filePath string, privateKey ed25519.PrivateKey) (*Artifact, error) {
	// Compute hash of the plaintext model
	hash, err := vorncrypto.HashFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("hash model: %w", err)
	}

	// Sign the hash
	sig, _, err := vorncrypto.SignFile(filePath, privateKey)
	if err != nil {
		return nil, fmt.Errorf("sign model: %w", err)
	}

	// Read file info
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat model: %w", err)
	}

	// Generate artifact ID from name + hash
	id := hex.EncodeToString([]byte(hash[:16]))

	a := &Artifact{
		ID:         id,
		Name:       name,
		Version:    version,
		Format:     format,
		SHA256:     hash,
		Signature:  sig,
		SizeBytes:  info.Size(),
		Encrypted:  s.dek != nil,
		ImportedAt: time.Now(),
		Verified:   true,
	}

	// Create blob directory
	blobDir := filepath.Join(s.baseDir, "blobs", id)
	if err := os.MkdirAll(blobDir, 0700); err != nil {
		return nil, fmt.Errorf("create blob dir: %w", err)
	}

	// Write hash file
	if err := os.WriteFile(filepath.Join(blobDir, "blob.sha256"), []byte(hash), 0600); err != nil {
		return nil, fmt.Errorf("write hash: %w", err)
	}

	// Write signature
	if err := os.WriteFile(filepath.Join(blobDir, "blob.sig"), sig, 0600); err != nil {
		return nil, fmt.Errorf("write signature: %w", err)
	}

	// Store model blob (encrypted or plain)
	blobPath := filepath.Join(blobDir, "blob.enc")
	if s.dek != nil {
		if err := vorncrypto.EncryptFile(filePath, blobPath, s.dek, id); err != nil {
			return nil, fmt.Errorf("encrypt model: %w", err)
		}
	} else {
		// Copy plaintext
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(blobPath, data, 0600); err != nil {
			return nil, err
		}
		a.Encrypted = false
	}

	// Write metadata
	metaJSON, _ := json.MarshalIndent(a, "", "  ")
	if err := os.WriteFile(filepath.Join(blobDir, "metadata.json"), metaJSON, 0600); err != nil {
		return nil, fmt.Errorf("write metadata: %w", err)
	}

	s.mu.Lock()
	s.artifacts[a.ID] = a
	s.mu.Unlock()

	s.saveIndex()
	return a, nil
}

// Verify checks that a model's signature and hash are valid against trusted keys.
func (s *Store) Verify(id string) error {
	s.mu.RLock()
	a, ok := s.artifacts[id]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("model %q not found", id)
	}

	if len(s.trustKeys) == 0 {
		return fmt.Errorf("no trusted public keys loaded")
	}

	_, err := vorncrypto.Verify(a.SHA256, a.Signature, s.trustKeys)
	if err != nil {
		a.Verified = false
		return fmt.Errorf("model %s: %w", id, err)
	}

	a.Verified = true
	return nil
}

// MustVerify loads and verifies a model, refusing to return it if invalid.
func (s *Store) MustVerify(id string) (*Artifact, error) {
	if err := s.Verify(id); err != nil {
		return nil, err
	}
	return s.Get(id)
}

// Get retrieves a model by ID.
func (s *Store) Get(id string) (*Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.artifacts[id]
	if !ok {
		return nil, fmt.Errorf("model %q not found", id)
	}
	return a, nil
}

// DecryptTo decrypts a model blob to the given destination path.
func (s *Store) DecryptTo(id, dstPath string) error {
	a, err := s.Get(id)
	if err != nil {
		return err
	}

	blobPath := filepath.Join(s.baseDir, "blobs", id, "blob.enc")
	if a.Encrypted && s.dek != nil {
		return vorncrypto.DecryptFile(blobPath, dstPath, s.dek, id)
	}

	// Not encrypted — just copy
	data, err := os.ReadFile(blobPath)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, data, 0600)
}

// List returns all artifacts.
func (s *Store) List() []*Artifact {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Artifact, 0, len(s.artifacts))
	for _, a := range s.artifacts {
		out = append(out, a)
	}
	return out
}

// Remove deletes a model from the store and removes its files.
func (s *Store) Remove(id string) error {
	s.mu.Lock()
	if _, ok := s.artifacts[id]; !ok {
		s.mu.Unlock()
		return fmt.Errorf("model %q not found", id)
	}
	delete(s.artifacts, id)
	s.mu.Unlock()

	blobDir := filepath.Join(s.baseDir, "blobs", id)
	os.RemoveAll(blobDir)

	s.saveIndex()
	return nil
}

// loadIndex reads the model index from disk into memory.
func (s *Store) loadIndex() {
	indexPath := filepath.Join(s.baseDir, "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return
	}
	var artifacts []*Artifact
	if err := json.Unmarshal(data, &artifacts); err != nil {
		return
	}
	for _, a := range artifacts {
		s.artifacts[a.ID] = a
	}
}

// saveIndex writes the model index to disk.
func (s *Store) saveIndex() {
	s.mu.RLock()
	artifacts := make([]*Artifact, 0, len(s.artifacts))
	for _, a := range s.artifacts {
		artifacts = append(artifacts, a)
	}
	s.mu.RUnlock()

	data, _ := json.MarshalIndent(artifacts, "", "  ")
	indexPath := filepath.Join(s.baseDir, "index.json")
	os.MkdirAll(filepath.Dir(indexPath), 0700)
	os.WriteFile(indexPath, data, 0600)
}
