package isolation

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// apptainerLauncher runs jobs inside Apptainer containers.
// Used for both appliance (minimal isolation) and research (full containment) modes.
type apptainerLauncher struct {
	research bool // true = --contain --writable-tmpfs; false = minimal flags
}

func (a *apptainerLauncher) Name() string {
	if a.research {
		return "apptainer-research"
	}
	return "apptainer-appliance"
}

func (a *apptainerLauncher) Launch(ctx context.Context, spec *JobSpec) (*Result, error) {
	args := []string{"exec"}

	// GPU passthrough
	args = append(args, "--nv")

	// Set visible GPU devices
	if len(spec.GPUDevices) > 0 {
		devs := make([]string, len(spec.GPUDevices))
		for i, d := range spec.GPUDevices {
			devs[i] = strconv.Itoa(d)
		}
		args = append(args, "--env", "CUDA_VISIBLE_DEVICES="+strings.Join(devs, ","))
	}

	// Research mode: full containment
	if a.research {
		args = append(args, "--contain", "--writable-tmpfs")
	}

	// Bind mount model into container
	if spec.ModelPath != "" {
		args = append(args, "--bind", spec.ModelPath+":/model:ro")
	}

	// Bind mount workspace
	if spec.WorkspaceDir != "" {
		args = append(args, "--bind", spec.WorkspaceDir+":/workspace")
	}

	// Inject environment variables
	for k, v := range spec.EnvVars {
		args = append(args, "--env", k+"="+v)
	}

	// CPU limit via cgroup (if available)
	if spec.CPULimit > 0 {
		args = append(args, "--cpus", strconv.Itoa(spec.CPULimit))
	}

	// Runtime image
	args = append(args, spec.RuntimeImage)

	// Command
	args = append(args, spec.Command...)

	cmd := exec.CommandContext(ctx, "apptainer", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set process group for cleanup
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	err := cmd.Run()

	result := &Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return result, fmt.Errorf("apptainer exec: %w", err)
		}
	}

	return result, nil
}
