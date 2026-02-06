package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestSignAndVerify(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	data := []byte("hello vorn os model payload")

	sig := Sign(data, priv)
	if len(sig) != ed25519.SignatureSize {
		t.Fatalf("expected signature size %d, got %d", ed25519.SignatureSize, len(sig))
	}

	ok := Verify(data, sig, []ed25519.PublicKey{pub})
	if !ok {
		t.Error("Verify returned false for valid signature")
	}

	// Wrong key should fail
	pub2, _, _ := ed25519.GenerateKey(rand.Reader)
	ok = Verify(data, sig, []ed25519.PublicKey{pub2})
	if ok {
		t.Error("Verify returned true for wrong key")
	}

	// Tampered data should fail
	ok = Verify([]byte("tampered"), sig, []ed25519.PublicKey{pub})
	if ok {
		t.Error("Verify returned true for tampered data")
	}
}

func TestHashFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.bin")
	if err := os.WriteFile(f, []byte("test content for hashing"), 0644); err != nil {
		t.Fatal(err)
	}

	hash, err := HashFile(f)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	if len(hash) != 64 { // hex-encoded SHA-256
		t.Errorf("expected 64 char hex hash, got %d chars", len(hash))
	}

	// Same content should produce same hash
	hash2, _ := HashFile(f)
	if hash != hash2 {
		t.Error("HashFile not deterministic")
	}
}

func TestSignFile(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)

	dir := t.TempDir()
	f := filepath.Join(dir, "model.bin")
	if err := os.WriteFile(f, []byte("model weights go here"), 0644); err != nil {
		t.Fatal(err)
	}

	sig, hash, err := SignFile(f, priv)
	if err != nil {
		t.Fatalf("SignFile: %v", err)
	}
	if len(sig) != ed25519.SignatureSize {
		t.Errorf("expected sig size %d, got %d", ed25519.SignatureSize, len(sig))
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
}
