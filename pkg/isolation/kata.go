package isolation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// kataLauncher runs jobs inside Kata lightweight VMs.
// Provides hardware-level isolation via VFIO GPU passthrough.
type kataLauncher struct{}

func (k *kataLauncher) Name() string { return "kata" }

func (k *kataLauncher) Launch(ctx context.Context, spec *JobSpec) (*Result, error) {
	// Prepare OCI bundle for kata-runtime
	bundleDir, err := k.prepareBundle(spec)
	if err != nil {
		return nil, fmt.Errorf("prepare kata bundle: %w", err)
	}
	defer os.RemoveAll(bundleDir)

	// Build kata-runtime command
	containerID := "vorn-" + spec.JobID
	args := []string{
		"run",
		"--bundle", bundleDir,
		containerID,
	}

	cmd := exec.CommandContext(ctx, "kata-runtime", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	result := &Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return result, fmt.Errorf("kata-runtime: %w", err)
		}
	}

	// Cleanup container
	exec.CommandContext(ctx, "kata-runtime", "delete", containerID).Run()

	return result, nil
}

// prepareBundle creates a minimal OCI runtime bundle for Kata.
func (k *kataLauncher) prepareBundle(spec *JobSpec) (string, error) {
	bundleDir, err := os.MkdirTemp("", "vorn-kata-")
	if err != nil {
		return "", err
	}

	rootfsDir := filepath.Join(bundleDir, "rootfs")
	if err := os.MkdirAll(rootfsDir, 0755); err != nil {
		return "", err
	}

	// Build OCI config.json
	config := k.buildOCIConfig(spec)
	configJSON, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(filepath.Join(bundleDir, "config.json"), configJSON, 0644); err != nil {
		return "", err
	}

	return bundleDir, nil
}

// buildOCIConfig generates a minimal OCI runtime spec for Kata.
func (k *kataLauncher) buildOCIConfig(spec *JobSpec) map[string]interface{} {
	env := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"TERM=xterm",
	}

	if len(spec.GPUDevices) > 0 {
		devs := make([]string, len(spec.GPUDevices))
		for i, d := range spec.GPUDevices {
			devs[i] = strconv.Itoa(d)
		}
		env = append(env, "CUDA_VISIBLE_DEVICES="+strings.Join(devs, ","))
		env = append(env, "NVIDIA_VISIBLE_DEVICES=all")
		env = append(env, "NVIDIA_DRIVER_CAPABILITIES=compute,utility")
	}

	for k, v := range spec.EnvVars {
		env = append(env, k+"="+v)
	}

	config := map[string]interface{}{
		"ociVersion": "1.0.2",
		"process": map[string]interface{}{
			"terminal": false,
			"user": map[string]interface{}{
				"uid": 0,
				"gid": 0,
			},
			"args": spec.Command,
			"env":  env,
			"cwd":  "/workspace",
		},
		"root": map[string]interface{}{
			"path":     "rootfs",
			"readonly": false,
		},
		"mounts": []map[string]interface{}{
			{
				"destination": "/model",
				"source":      spec.ModelPath,
				"options":     []string{"bind", "ro"},
			},
			{
				"destination": "/workspace",
				"source":      spec.WorkspaceDir,
				"options":     []string{"bind", "rw"},
			},
		},
		"linux": map[string]interface{}{
			"resources": map[string]interface{}{
				"cpu": map[string]interface{}{
					"quota":  spec.CPULimit * 100000,
					"period": 100000,
				},
			},
		},
	}

	return config
}
