package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	if cfg.APIAddr != ":9800" {
		t.Errorf("expected API addr :9800, got %s", cfg.APIAddr)
	}
	if cfg.MetricsAddr != ":9801" {
		t.Errorf("expected metrics addr :9801, got %s", cfg.MetricsAddr)
	}
	if cfg.DefaultIsolation != "apptainer" {
		t.Errorf("expected default isolation apptainer, got %s", cfg.DefaultIsolation)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected log level info, got %s", cfg.LogLevel)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "test.yaml")

	content := `
api_addr: ":8080"
metrics_addr: ":8081"
log_level: "debug"
db_path: "/tmp/test.db"
runtime_dir: "/tmp/runtimes"
model_dir: "/tmp/models"
`
	if err := os.WriteFile(cfgFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.APIAddr != ":8080" {
		t.Errorf("expected :8080, got %s", cfg.APIAddr)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected debug, got %s", cfg.LogLevel)
	}
}

func TestValidate(t *testing.T) {
	cfg := Defaults()

	// Defaults should be valid
	if err := cfg.Validate(); err != nil {
		t.Errorf("Defaults should be valid: %v", err)
	}

	// Empty API addr should fail
	cfg.APIAddr = ""
	if err := cfg.Validate(); err == nil {
		t.Error("empty APIAddr should fail validation")
	}
}

func TestTLSEnabled(t *testing.T) {
	cfg := Defaults()
	if cfg.TLSEnabled() {
		t.Error("TLS should be disabled by default")
	}

	cfg.TLSCert = "/etc/vorn/tls/server.crt"
	cfg.TLSKey = "/etc/vorn/tls/server.key"
	if !cfg.TLSEnabled() {
		t.Error("TLS should be enabled when cert and key are set")
	}
}
