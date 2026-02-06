// Package isolation abstracts container/VM runtimes for job execution.
// It provides a common Launcher interface backed by Apptainer, Kata, or direct exec.
package isolation

import (
	"context"
	"fmt"
)

// Mode determines which isolation backend to use.
type Mode string

const (
	ModeAppliance Mode = "appliance" // minimal isolation, direct exec
	ModeResearch  Mode = "research"  // Apptainer container with containment
	ModeSecure    Mode = "secure"    // Kata lightweight VM with VFIO
)

// JobSpec describes what to launch.
type JobSpec struct {
	JobID        string
	RuntimeImage string            // path to SIF or OCI image
	ModelPath    string            // path to decrypted model on host
	WorkspaceDir string            // per-job workspace directory
	Command      []string          // entrypoint + args
	GPUDevices   []int             // NVIDIA device indices to expose
	VRAMLimit    int64             // MB (advisory)
	CPULimit     int               // cores (0 = unlimited)
	EnvVars      map[string]string // environment variables to inject
}

// Result is returned when a job finishes.
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// Launcher is the interface that isolation backends implement.
type Launcher interface {
	// Launch starts a job and blocks until it completes or ctx is cancelled.
	Launch(ctx context.Context, spec *JobSpec) (*Result, error)
	// Name returns the backend name for logging.
	Name() string
}

// NewLauncher returns the appropriate launcher for the given mode.
func NewLauncher(mode Mode) (Launcher, error) {
	switch mode {
	case ModeAppliance:
		return &applianceLauncher{}, nil
	case ModeResearch:
		return &apptainerLauncher{research: true}, nil
	case ModeSecure:
		return &kataLauncher{}, nil
	default:
		return nil, fmt.Errorf("unknown isolation mode: %s", mode)
	}
}
