// keyring.go provides integration with the Linux kernel keyring
// for secure in-memory storage of the data encryption key (DEK).
//
// On non-Linux systems, these functions fall back to environment variables.
package crypto

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const keyringDescription = "vorn-dek"

// StoreDEKInKeyring stores the DEK in the user session keyring.
// Uses keyctl(1) which is available on most Linux systems with keyutils.
func StoreDEKInKeyring(dek []byte) error {
	hexDEK := hex.EncodeToString(dek)

	cmd := exec.Command("keyctl", "add", "user", keyringDescription, hexDEK, "@u")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keyctl add: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// LoadDEKFromKeyring retrieves the DEK from the user session keyring.
func LoadDEKFromKeyring() ([]byte, error) {
	// First, find the key ID
	cmd := exec.Command("keyctl", "search", "@u", "user", keyringDescription)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("keyctl search: DEK not found in keyring (run vorn-model-unlocker first)")
	}

	keyID := strings.TrimSpace(string(out))

	// Read the key value
	cmd = exec.Command("keyctl", "pipe", keyID)
	out, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("keyctl pipe: %w", err)
	}

	dek, err := hex.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		return nil, fmt.Errorf("decode DEK from keyring: %w", err)
	}

	if len(dek) != DEKSize {
		return nil, fmt.Errorf("DEK in keyring has wrong size: %d (expected %d)", len(dek), DEKSize)
	}

	return dek, nil
}

// ClearDEKFromKeyring removes the DEK from the keyring.
func ClearDEKFromKeyring() error {
	cmd := exec.Command("keyctl", "search", "@u", "user", keyringDescription)
	out, err := cmd.Output()
	if err != nil {
		return nil // key not found, nothing to clear
	}

	keyID := strings.TrimSpace(string(out))
	cmd = exec.Command("keyctl", "unlink", keyID, "@u")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("keyctl unlink: %w", err)
	}
	return nil
}

// LoadDEKFromEnv loads the DEK from an environment variable fallback.
// Used on non-Linux systems or when keyring is unavailable.
func LoadDEKFromEnv() ([]byte, error) {
	hexDEK := os.Getenv("VORN_DEK")
	if hexDEK == "" {
		return nil, fmt.Errorf("VORN_DEK environment variable not set")
	}
	dek, err := hex.DecodeString(hexDEK)
	if err != nil {
		return nil, fmt.Errorf("decode VORN_DEK: %w", err)
	}
	if len(dek) != DEKSize {
		return nil, fmt.Errorf("VORN_DEK has wrong size: %d", len(dek))
	}
	return dek, nil
}
