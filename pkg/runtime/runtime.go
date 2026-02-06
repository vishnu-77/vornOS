// Package runtime manages immutable runtime layers — versioned bundles
// containing CUDA, cuDNN, AI frameworks, and minimal OS userland.
package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Layer represents a registered runtime layer.
type Layer struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Version       string    `json:"version"`
	ImageRef      string    `json:"image_ref"`       // OCI ref or path to SIF
	ImageFormat   string    `json:"image_format"`    // "oci" or "sif"
	CUDAVersion   string    `json:"cuda_version"`
	CuDNNVersion  string    `json:"cudnn_version"`
	Frameworks    []string  `json:"frameworks"`      // e.g. ["pytorch-2.2", "triton-2.1"]
	Architectures []string  `json:"architectures"`   // e.g. ["sm_80", "sm_90"]
	DriverMin     string    `json:"driver_min"`      // minimum compatible driver version
	SHA256        string    `json:"sha256"`
	RegisteredAt  time.Time `json:"registered_at"`
	Validated     bool      `json:"validated"`
}

// Store manages runtime layers on disk.
type Store struct {
	baseDir string
	layers  map[string]*Layer // keyed by ID
}

// NewStore creates a runtime store rooted at baseDir.
func NewStore(baseDir string) *Store {
	return &Store{
		baseDir: baseDir,
		layers:  make(map[string]*Layer),
	}
}

// Register adds a new runtime layer after computing its hash.
func (s *Store) Register(l *Layer, imagePath string) error {
	// TODO: read image from imagePath, compute sha256, store metadata
	_ = imagePath
	if l.ID == "" {
		l.ID = generateID(l.Name, l.Version)
	}
	l.RegisteredAt = time.Now()
	s.layers[l.ID] = l
	return nil
}

// Get retrieves a layer by ID.
func (s *Store) Get(id string) (*Layer, error) {
	l, ok := s.layers[id]
	if !ok {
		return nil, fmt.Errorf("runtime layer %q not found", id)
	}
	return l, nil
}

// List returns all registered layers.
func (s *Store) List() []*Layer {
	out := make([]*Layer, 0, len(s.layers))
	for _, l := range s.layers {
		out = append(out, l)
	}
	return out
}

// IsCompatible checks if a layer is compatible with the given driver and GPU arch.
func (s *Store) IsCompatible(layerID, driverVer, gpuArch string) (bool, error) {
	l, err := s.Get(layerID)
	if err != nil {
		return false, err
	}
	// TODO: proper semver comparison for driver
	_ = driverVer
	for _, arch := range l.Architectures {
		if arch == gpuArch {
			return true, nil
		}
	}
	return false, nil
}

func generateID(name, version string) string {
	h := sha256.Sum256([]byte(name + ":" + version))
	return hex.EncodeToString(h[:8])
}
