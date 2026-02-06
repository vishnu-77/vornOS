package compat

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMatrix(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "gpu-compat.yaml")

	content := `
entries:
  - min_driver: "535.0"
    max_driver: "535.999"
    max_cuda: "12.2"
    architectures: ["sm_70", "sm_75", "sm_80", "sm_86", "sm_89", "sm_90"]
    cudnn_versions: ["8.9"]
    notes: "Test entry"
  - min_driver: "550.0"
    max_driver: "550.999"
    max_cuda: "12.4"
    architectures: ["sm_70", "sm_75", "sm_80", "sm_86", "sm_89", "sm_90"]
    cudnn_versions: ["9.0"]
    notes: "Newer driver"
`
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadMatrix(f)
	if err != nil {
		t.Fatalf("LoadMatrix: %v", err)
	}

	if len(m.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m.Entries))
	}

	// Driver 535.54 should be compatible
	if !m.IsDriverCompatible("535.54") {
		t.Error("driver 535.54 should be compatible")
	}

	// Driver 550.10 should be compatible
	if !m.IsDriverCompatible("550.10") {
		t.Error("driver 550.10 should be compatible")
	}

	// Driver 500.0 should not be compatible
	if m.IsDriverCompatible("500.0") {
		t.Error("driver 500.0 should not be compatible")
	}

	// sm_80 should be supported for driver 535.54
	archs := m.SupportedArchitectures("535.54")
	found := false
	for _, a := range archs {
		if a == "sm_80" {
			found = true
			break
		}
	}
	if !found {
		t.Error("sm_80 should be supported for driver 535.54")
	}
}
