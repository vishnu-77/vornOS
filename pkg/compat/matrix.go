// Package compat loads and queries the GPU driver/CUDA/cuDNN compatibility matrix.
package compat

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Matrix is the top-level compatibility data.
type Matrix struct {
	Version     int                    `yaml:"version"`
	Drivers     map[string]DriverInfo  `yaml:"drivers"`
	CUDAToolkit map[string]CUDAInfo    `yaml:"cuda_toolkit"`
	GPUModels   map[string]GPUModelInfo `yaml:"gpu_models"`
}

// DriverInfo describes a specific NVIDIA driver version.
type DriverInfo struct {
	ReleaseDate            string   `yaml:"release_date"`
	CUDAMax                string   `yaml:"cuda_max"`
	SupportedArchitectures []string `yaml:"supported_architectures"`
	EOL                    bool     `yaml:"eol"`
}

// CUDAInfo describes a CUDA toolkit version.
type CUDAInfo struct {
	MinDriver    string              `yaml:"min_driver"`
	CuDNNVersions []string           `yaml:"cudnn_versions"`
	Frameworks   map[string][]string `yaml:"frameworks"`
}

// GPUModelInfo describes a GPU hardware model.
type GPUModelInfo struct {
	Architecture string `yaml:"architecture"`
	VRAMMB       int64  `yaml:"vram_mb"`
	PCIeGen      int    `yaml:"pcie_gen"`
	NVLink       bool   `yaml:"nvlink"`
	MIGCapable   bool   `yaml:"mig_capable"`
}

// LoadMatrix reads the compatibility matrix from a YAML file.
func LoadMatrix(path string) (*Matrix, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read matrix: %w", err)
	}
	var m Matrix
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse matrix: %w", err)
	}
	return &m, nil
}

// IsDriverCompatible checks if a driver version supports a given CUDA version.
func (m *Matrix) IsDriverCompatible(driverVer, cudaVer string) bool {
	info, ok := m.Drivers[driverVer]
	if !ok {
		return false
	}
	return compareSemver(info.CUDAMax, cudaVer) >= 0
}

// SupportedArchitectures returns the GPU architectures supported by a driver.
func (m *Matrix) SupportedArchitectures(driverVer string) []string {
	info, ok := m.Drivers[driverVer]
	if !ok {
		return nil
	}
	return info.SupportedArchitectures
}

// ValidateLayer checks if a runtime layer is compatible with the installed driver.
// Returns a list of warnings/errors (empty = compatible).
func (m *Matrix) ValidateLayer(layerCUDAVer string, layerArchs []string, driverVer string) []string {
	var issues []string

	driverInfo, ok := m.Drivers[driverVer]
	if !ok {
		issues = append(issues, fmt.Sprintf("unknown driver version %s", driverVer))
		return issues
	}

	if driverInfo.EOL {
		issues = append(issues, fmt.Sprintf("driver %s is end-of-life", driverVer))
	}

	if compareSemver(driverInfo.CUDAMax, layerCUDAVer) < 0 {
		issues = append(issues, fmt.Sprintf("driver %s supports CUDA up to %s, but layer requires %s",
			driverVer, driverInfo.CUDAMax, layerCUDAVer))
	}

	supportedArchs := make(map[string]bool)
	for _, a := range driverInfo.SupportedArchitectures {
		supportedArchs[a] = true
	}
	for _, arch := range layerArchs {
		if !supportedArchs[arch] {
			issues = append(issues, fmt.Sprintf("architecture %s not supported by driver %s", arch, driverVer))
		}
	}

	return issues
}

// compareSemver does a simple major.minor version comparison.
// Returns -1, 0, or 1 like strings.Compare.
func compareSemver(a, b string) int {
	aParts := strings.SplitN(a, ".", 3)
	bParts := strings.SplitN(b, ".", 3)

	for i := 0; i < 2; i++ {
		aVal, bVal := "0", "0"
		if i < len(aParts) {
			aVal = aParts[i]
		}
		if i < len(bParts) {
			bVal = bParts[i]
		}
		cmp := strings.Compare(padLeft(aVal, 5), padLeft(bVal, 5))
		if cmp != 0 {
			return cmp
		}
	}
	return 0
}

func padLeft(s string, n int) string {
	for len(s) < n {
		s = "0" + s
	}
	return s
}
