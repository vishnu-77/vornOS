// Package hooks executes pre/post job lifecycle hooks.
// Hooks are shell scripts in /opt/vorn/libexec/hooks/ that run
// before and after job execution for GPU reset, cleanup, etc.
package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

const (
	defaultHooksDir = "/opt/vorn/libexec/hooks"
	hookTimeout     = 30 * time.Second
)

// RunPreJob executes the pre-job hook (GPU reset, VRAM clear).
// jobID and gpuDevices are passed as environment variables.
func RunPreJob(hooksDir, jobID string, gpuDevices []int) error {
	return runHook(hooksDir, "pre-job.sh", jobID, gpuDevices)
}

// RunPostJob executes the post-job hook (cleanup, artifact collection).
func RunPostJob(hooksDir, jobID string, gpuDevices []int) error {
	return runHook(hooksDir, "post-job.sh", jobID, gpuDevices)
}

func runHook(hooksDir, hookName, jobID string, gpuDevices []int) error {
	if hooksDir == "" {
		hooksDir = defaultHooksDir
	}

	path := hooksDir + "/" + hookName
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // hook not installed, skip silently
	}

	cmd := exec.Command("/bin/sh", path)
	cmd.Env = append(os.Environ(),
		"VORN_JOB_ID="+jobID,
		fmt.Sprintf("VORN_GPU_DEVICES=%v", gpuDevices),
	)

	// Enforce timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(hookTimeout):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return fmt.Errorf("hook %s timed out after %v", hookName, hookTimeout)
	}
}
