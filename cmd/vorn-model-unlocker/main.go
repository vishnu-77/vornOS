// vorn-model-unlocker unseals the model store data encryption key (DEK)
// at boot and stores it in the Linux kernel keyring.
//
// Modes:
//   --init        Generate a new DEK, wrap it with a passphrase, and store on disk.
//   (default)     Unseal existing DEK and load into kernel keyring.
package main

import (
	"flag"
	"fmt"
	"os"

	"golang.org/x/crypto/argon2"

	vorncrypto "github.com/vornos/buildos/pkg/crypto"
)

const (
	defaultKeyPath = "/var/lib/vorn/encrypt/model-store.key.enc"
	argon2Time     = 3
	argon2Memory   = 64 * 1024 // 64 MB
	argon2Threads  = 4
	argon2KeyLen   = 32
)

func main() {
	initMode := flag.Bool("init", false, "generate a new DEK and wrap it")
	keyPath := flag.String("key", defaultKeyPath, "path to the wrapped DEK file")
	flag.Parse()

	if *initMode {
		if err := initDEK(*keyPath); err != nil {
			fmt.Fprintf(os.Stderr, "init failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("DEK generated and wrapped successfully.")
		fmt.Printf("Stored at: %s\n", *keyPath)
		return
	}

	if err := unsealDEK(*keyPath); err != nil {
		fmt.Fprintf(os.Stderr, "unseal failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("DEK unsealed and loaded into kernel keyring.")
}

// initDEK generates a new DEK, derives a KEK from a passphrase, wraps the DEK, and stores it.
func initDEK(keyPath string) error {
	if _, err := os.Stat(keyPath); err == nil {
		return fmt.Errorf("key file already exists at %s; refusing to overwrite", keyPath)
	}

	// Generate DEK
	dek, err := vorncrypto.GenerateDEK()
	if err != nil {
		return err
	}

	// Get passphrase
	passphrase := os.Getenv("VORN_PASSPHRASE")
	if passphrase == "" {
		fmt.Print("Enter passphrase for model store encryption: ")
		_, err := fmt.Scanln(&passphrase)
		if err != nil {
			return fmt.Errorf("read passphrase: %w", err)
		}
	}

	// Derive KEK from passphrase using Argon2id
	salt := make([]byte, 16)
	if _, err := os.ReadFile("/dev/urandom"); err != nil {
		// Fallback: use crypto/rand via GenerateNonce
		nonce, _ := vorncrypto.GenerateNonce()
		copy(salt, nonce)
		extra, _ := vorncrypto.GenerateNonce()
		copy(salt[12:], extra[:4])
	}

	kek := argon2.IDKey([]byte(passphrase), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	// Wrap DEK with KEK
	wrapped, err := vorncrypto.WrapDEK(dek, kek)
	if err != nil {
		return fmt.Errorf("wrap DEK: %w", err)
	}

	// Store: salt(16) || wrapped DEK
	output := append(salt, wrapped...)
	os.MkdirAll("/var/lib/vorn/encrypt", 0700)
	if err := os.WriteFile(keyPath, output, 0600); err != nil {
		return fmt.Errorf("write key file: %w", err)
	}

	return nil
}

// unsealDEK reads the wrapped DEK, derives the KEK, unwraps, and stores in keyring.
func unsealDEK(keyPath string) error {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("read key file: %w", err)
	}

	if len(data) < 16 {
		return fmt.Errorf("key file too small")
	}

	salt := data[:16]
	wrapped := data[16:]

	// Get passphrase
	passphrase := os.Getenv("VORN_PASSPHRASE")
	if passphrase == "" {
		fmt.Print("Enter model store passphrase: ")
		_, err := fmt.Scanln(&passphrase)
		if err != nil {
			return fmt.Errorf("read passphrase: %w", err)
		}
	}

	// Derive KEK
	kek := argon2.IDKey([]byte(passphrase), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	// Unwrap DEK
	dek, err := vorncrypto.UnwrapDEK(wrapped, kek)
	if err != nil {
		return fmt.Errorf("unwrap DEK (wrong passphrase?): %w", err)
	}

	// Store in kernel keyring
	if err := vorncrypto.StoreDEKInKeyring(dek); err != nil {
		return fmt.Errorf("store in keyring: %w", err)
	}

	return nil
}
