package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPolicy(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.policy.yaml")

	content := `
name: test
max_gpus: 4
max_vram_mb: 49152
max_cpu_cores: 32
max_walltime_seconds: 86400
allow_network: false
allow_host_mounts: false
`
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	p, err := LoadPolicy(f)
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}

	if p.Name != "test" {
		t.Errorf("expected name test, got %s", p.Name)
	}
	if p.MaxGPUs != 4 {
		t.Errorf("expected max GPUs 4, got %d", p.MaxGPUs)
	}
	if p.AllowNetwork {
		t.Error("expected network denied")
	}
}

func TestLoadPolicySet(t *testing.T) {
	dir := t.TempDir()

	policies := map[string]string{
		"default.policy.yaml": `
name: default
max_gpus: 4
max_vram_mb: 49152
max_cpu_cores: 32
max_walltime_seconds: 86400
allow_network: false
allow_host_mounts: false
`,
		"research.policy.yaml": `
name: research
max_gpus: 8
max_vram_mb: 81920
max_cpu_cores: 64
max_walltime_seconds: 259200
allow_network: true
allow_host_mounts: false
`,
	}

	for name, content := range policies {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	ps, err := LoadPolicySet(dir)
	if err != nil {
		t.Fatalf("LoadPolicySet: %v", err)
	}

	p := ps.Get("default")
	if p == nil {
		t.Fatal("expected default policy")
	}
	if p.MaxGPUs != 4 {
		t.Errorf("expected 4, got %d", p.MaxGPUs)
	}

	p = ps.Get("research")
	if p == nil {
		t.Fatal("expected research policy")
	}
	if !p.AllowNetwork {
		t.Error("research policy should allow network")
	}

	p = ps.Get("nonexistent")
	if p != nil {
		t.Error("expected nil for nonexistent policy")
	}
}
