package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	dek := GenerateDEK()
	if len(dek) != 32 {
		t.Fatalf("expected 32-byte DEK, got %d", len(dek))
	}

	plaintext := []byte("sensitive model weights data for encryption test")
	aad := []byte("model-001")

	ciphertext, err := Encrypt(plaintext, dek, aad)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if bytes.Equal(plaintext, ciphertext) {
		t.Error("ciphertext equals plaintext")
	}

	decrypted, err := Decrypt(ciphertext, dek, aad)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Error("decrypted data doesn't match original")
	}

	// Wrong DEK should fail
	wrongDEK := GenerateDEK()
	_, err = Decrypt(ciphertext, wrongDEK, aad)
	if err == nil {
		t.Error("Decrypt with wrong DEK should fail")
	}

	// Wrong AAD should fail
	_, err = Decrypt(ciphertext, dek, []byte("wrong-model"))
	if err == nil {
		t.Error("Decrypt with wrong AAD should fail")
	}
}

func TestEncryptDecryptFile(t *testing.T) {
	dek := GenerateDEK()
	dir := t.TempDir()

	src := filepath.Join(dir, "model.bin")
	enc := filepath.Join(dir, "model.enc")
	dec := filepath.Join(dir, "model.dec")

	original := []byte("model file content for file-level encryption test")
	if err := os.WriteFile(src, original, 0644); err != nil {
		t.Fatal(err)
	}

	if err := EncryptFile(src, enc, dek, []byte("model-002")); err != nil {
		t.Fatalf("EncryptFile: %v", err)
	}

	if err := DecryptFile(enc, dec, dek, []byte("model-002")); err != nil {
		t.Fatalf("DecryptFile: %v", err)
	}

	decContent, err := os.ReadFile(dec)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(original, decContent) {
		t.Error("decrypted file content doesn't match original")
	}
}

func TestWrapUnwrapDEK(t *testing.T) {
	dek := GenerateDEK()
	passphrase := "test-passphrase-for-wrapping"

	wrapped, err := WrapDEK(dek, passphrase)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}

	unwrapped, err := UnwrapDEK(wrapped, passphrase)
	if err != nil {
		t.Fatalf("UnwrapDEK: %v", err)
	}

	if !bytes.Equal(dek, unwrapped) {
		t.Error("unwrapped DEK doesn't match original")
	}

	// Wrong passphrase should fail
	_, err = UnwrapDEK(wrapped, "wrong-passphrase")
	if err == nil {
		t.Error("UnwrapDEK with wrong passphrase should fail")
	}
}
