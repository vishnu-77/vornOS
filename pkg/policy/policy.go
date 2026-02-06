// Package policy defines and enforces execution policies for jobs.
// Policies set per-isolation-mode resource limits and access controls.
package policy

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Policy defines resource and access limits for an isolation mode.
type Policy struct {
	Name           string `yaml:"name"`
	MaxGPUs        int    `yaml:"max_gpus"`
	MaxVRAMMB      int64  `yaml:"max_vram_mb"`      // 0 = no limit
	MaxCPUCores    int    `yaml:"max_cpu_cores"`     // 0 = no limit
	MaxWalltimeSec int    `yaml:"max_walltime_sec"`  // 0 = no limit
	NetworkAllowed bool   `yaml:"network_allowed"`
	MountsAllowed  bool   `yaml:"mounts_allowed"`    // allow additional bind mounts
}

// JobRequest is the subset of job fields relevant to policy evaluation.
type JobRequest struct {
	GPULimit  int
	VRAMLimit int64
	CPULimit  int
	Mode      string
}

// LoadPolicy reads a policy from a YAML file.
func LoadPolicy(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read policy: %w", err)
	}
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse policy: %w", err)
	}
	return &p, nil
}

// Validate checks if a job request satisfies the policy constraints.
// Returns nil if the request is within limits.
func (p *Policy) Validate(req JobRequest) error {
	if p.MaxGPUs > 0 && req.GPULimit > p.MaxGPUs {
		return fmt.Errorf("policy %q: gpu_limit %d exceeds max %d", p.Name, req.GPULimit, p.MaxGPUs)
	}
	if p.MaxVRAMMB > 0 && req.VRAMLimit > p.MaxVRAMMB {
		return fmt.Errorf("policy %q: vram_limit %d MB exceeds max %d MB", p.Name, req.VRAMLimit, p.MaxVRAMMB)
	}
	if p.MaxCPUCores > 0 && req.CPULimit > p.MaxCPUCores {
		return fmt.Errorf("policy %q: cpu_limit %d exceeds max %d", p.Name, req.CPULimit, p.MaxCPUCores)
	}
	return nil
}

// PolicySet holds policies keyed by isolation mode name.
type PolicySet struct {
	policies map[string]*Policy
}

// LoadPolicyDir loads all .policy.yaml files from a directory.
func LoadPolicyDir(dir string) (*PolicySet, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return &PolicySet{policies: make(map[string]*Policy)}, nil
		}
		return nil, err
	}

	ps := &PolicySet{policies: make(map[string]*Policy)}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p, err := LoadPolicy(dir + "/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", e.Name(), err)
		}
		ps.policies[p.Name] = p
	}
	return ps, nil
}

// Get returns the policy for the given mode, or nil if no policy is defined.
func (ps *PolicySet) Get(mode string) *Policy {
	return ps.policies[mode]
}

// Validate checks a job request against the appropriate policy for its mode.
func (ps *PolicySet) Validate(req JobRequest) error {
	p := ps.policies[req.Mode]
	if p == nil {
		return nil // no policy = no restrictions
	}
	return p.Validate(req)
}
