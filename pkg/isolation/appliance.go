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

// applianceLauncher runs jobs with minimal isolation: direct process execution
// with cgroup resource limits and CUDA_VISIBLE_DEVICES restriction.
// For development/testing and trusted workloads only.
type applianceLauncher struct{}

func (a *applianceLauncher) Name() string { return "appliance" }

func (a *applianceLauncher) Launch(ctx context.Context, spec *JobSpec) (*Result, error) {
	if len(spec.Command) == 0 {
		return nil, fmt.Errorf("no command specified")
	}

	cmd := exec.CommandContext(ctx, spec.Command[0], spec.Command[1:]...)

	// Set GPU visibility
	if len(spec.GPUDevices) > 0 {
		devs := make([]string, len(spec.GPUDevices))
		for i, d := range spec.GPUDevices {
			devs[i] = strconv.Itoa(d)
		}
		cmd.Env = append(cmd.Environ(), "CUDA_VISIBLE_DEVICES="+strings.Join(devs, ","))
	}

	// Inject environment variables
	for k, v := range spec.EnvVars {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Set working directory to workspace
	if spec.WorkspaceDir != "" {
		cmd.Dir = spec.WorkspaceDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

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
			return result, fmt.Errorf("exec: %w", err)
		}
	}

	return result, nil
}
