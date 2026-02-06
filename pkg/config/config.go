// Package config handles loading and validation of the VRM daemon configuration.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Version is set at build time via ldflags.
var Version = "dev"

// Config is the top-level daemon configuration.
type Config struct {
	// Network addresses
	APIAddr     string `yaml:"api_addr"`      // gRPC API listen address (TCP)
	UnixSocket  string `yaml:"unix_socket"`   // gRPC API Unix domain socket (local fast path)
	MetricsAddr string `yaml:"metrics_addr"`  // Prometheus metrics HTTP address

	// TLS (for gRPC over TCP; Unix socket uses filesystem permissions)
	TLSCert string `yaml:"tls_cert"` // path to server certificate
	TLSKey  string `yaml:"tls_key"`  // path to server private key
	TLSCA   string `yaml:"tls_ca"`   // path to CA certificate for client verification (mTLS)

	// Storage paths
	DBPath     string `yaml:"db_path"`      // SQLite database path
	RuntimeDir string `yaml:"runtime_dir"`  // runtime layer storage root
	ModelDir   string `yaml:"model_dir"`    // encrypted model store root
	JobDir     string `yaml:"job_dir"`      // per-job workspace root
	LogDir     string `yaml:"log_dir"`      // structured log directory

	// Trust and signing
	TrustPublicKeyPath string `yaml:"trust_public_key"` // primary Ed25519 public key
	TrustDir           string `yaml:"trust_dir"`         // directory of trusted public keys (rotation)

	// Policies
	PolicyDir string `yaml:"policy_dir"` // execution policy directory

	// GPU
	GPUDriverMinVersion string `yaml:"gpu_driver_min_version"` // minimum NVIDIA driver version
	GPUCompatMatrix     string `yaml:"gpu_compat_matrix"`      // path to compatibility matrix YAML
	GPUHealthPollSec    int    `yaml:"gpu_health_poll_sec"`    // GPU health polling interval

	// Isolation defaults
	DefaultIsolation string `yaml:"default_isolation"` // appliance | research | secure

	// Logging
	LogLevel string `yaml:"log_level"` // debug | info | warn | error

	// Watchdog
	WatchdogSec int `yaml:"watchdog_sec"` // systemd watchdog interval (0 = disabled)
}

// Defaults returns a Config with sane defaults for production.
func Defaults() *Config {
	return &Config{
		APIAddr:     "127.0.0.1:9800",
		UnixSocket:  "/var/run/vorn/vornd.sock",
		MetricsAddr: "0.0.0.0:9801",

		TLSCert: "",
		TLSKey:  "",
		TLSCA:   "",

		DBPath:     "/var/lib/vorn/db/vorn.db",
		RuntimeDir: "/var/lib/vorn/runtimes",
		ModelDir:   "/var/lib/vorn/models",
		JobDir:     "/var/lib/vorn/jobs",
		LogDir:     "/var/log/vorn",

		TrustPublicKeyPath: "/etc/vorn/trust.pub",
		TrustDir:           "/etc/vorn/trust.d",

		PolicyDir: "/etc/vorn/policies",

		GPUDriverMinVersion: "535.0",
		GPUCompatMatrix:     "/opt/vorn/lib/gpu-compat/matrix.yaml",
		GPUHealthPollSec:    5,

		DefaultIsolation: "appliance",

		LogLevel: "info",

		WatchdogSec: 30,
	}
}

// Load reads config from a YAML file, falling back to defaults for missing fields.
func Load(path string) (*Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // use defaults if no config file
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

// Validate checks that required fields have valid values.
func (c *Config) Validate() error {
	if c.APIAddr == "" && c.UnixSocket == "" {
		return fmt.Errorf("at least one of api_addr or unix_socket must be set")
	}

	validIsolation := map[string]bool{"appliance": true, "research": true, "secure": true}
	if !validIsolation[c.DefaultIsolation] {
		return fmt.Errorf("invalid default_isolation %q: must be appliance, research, or secure", c.DefaultIsolation)
	}

	validLogLevel := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevel[c.LogLevel] {
		return fmt.Errorf("invalid log_level %q: must be debug, info, warn, or error", c.LogLevel)
	}

	if c.GPUHealthPollSec < 1 {
		c.GPUHealthPollSec = 5
	}

	// TLS: if any TLS field is set, all three must be set
	tlsFields := []string{c.TLSCert, c.TLSKey, c.TLSCA}
	tlsSet := 0
	for _, f := range tlsFields {
		if f != "" {
			tlsSet++
		}
	}
	if tlsSet > 0 && tlsSet < 3 {
		return fmt.Errorf("tls_cert, tls_key, and tls_ca must all be set or all be empty")
	}

	return nil
}

// TLSEnabled returns true if mTLS is configured.
func (c *Config) TLSEnabled() bool {
	return c.TLSCert != "" && c.TLSKey != "" && c.TLSCA != ""
}
