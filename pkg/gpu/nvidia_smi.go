package gpu

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// nvidiaSmiQueryFields defines the nvidia-smi --query-gpu fields we request.
const nvidiaSmiQueryFields = "index,uuid,name,driver_version,cuda_version," +
	"memory.total,memory.used,memory.free,temperature.gpu,utilization.gpu,gpu_bus_id"

// execNvidiaSmi runs nvidia-smi with the given arguments and returns stdout.
func execNvidiaSmi(args ...string) (string, error) {
	cmd := exec.Command("nvidia-smi", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("nvidia-smi failed (exit %d): %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("nvidia-smi exec: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// parseCSVInventory parses nvidia-smi --query-gpu CSV output into Device structs.
func parseCSVInventory(csv string) ([]Device, error) {
	lines := strings.Split(csv, "\n")
	var devices []Device

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, ", ")
		if len(fields) < 11 {
			return nil, fmt.Errorf("expected 11 fields, got %d: %s", len(fields), line)
		}

		idx, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil {
			return nil, fmt.Errorf("parse index %q: %w", fields[0], err)
		}

		totalVRAM, _ := strconv.ParseInt(strings.TrimSpace(fields[5]), 10, 64)
		usedVRAM, _ := strconv.ParseInt(strings.TrimSpace(fields[6]), 10, 64)
		freeVRAM, _ := strconv.ParseInt(strings.TrimSpace(fields[7]), 10, 64)
		temp, _ := strconv.Atoi(strings.TrimSpace(fields[8]))
		util, _ := strconv.Atoi(strings.TrimSpace(fields[9]))

		dev := Device{
			Index:        idx,
			UUID:         strings.TrimSpace(fields[1]),
			Name:         strings.TrimSpace(fields[2]),
			DriverVer:    strings.TrimSpace(fields[3]),
			CUDAVer:      strings.TrimSpace(fields[4]),
			TotalVRAM:    totalVRAM,
			UsedVRAM:     usedVRAM,
			FreeVRAM:     freeVRAM,
			Temperature:  temp,
			Utilization:  util,
			Architecture: busIDToArch(strings.TrimSpace(fields[10])),
		}
		devices = append(devices, dev)
	}
	return devices, nil
}

// discoverDevices runs nvidia-smi and returns the full GPU inventory.
func discoverDevices() (*Inventory, error) {
	out, err := execNvidiaSmi(
		"--query-gpu="+nvidiaSmiQueryFields,
		"--format=csv,noheader,nounits",
	)
	if err != nil {
		return nil, err
	}

	devices, err := parseCSVInventory(out)
	if err != nil {
		return nil, fmt.Errorf("parse gpu inventory: %w", err)
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("no NVIDIA GPUs found")
	}

	// Query compute capability separately (not available in --query-gpu)
	if archOut, err := execNvidiaSmi(
		"--query-gpu=index,compute_cap",
		"--format=csv,noheader,nounits",
	); err == nil {
		archMap := parseComputeCaps(archOut)
		for i := range devices {
			if arch, ok := archMap[devices[i].Index]; ok {
				devices[i].Architecture = arch
			}
		}
	}

	return &Inventory{
		Devices:   devices,
		Timestamp: time.Now(),
	}, nil
}

// refreshDevices updates dynamic fields (VRAM usage, utilization, temperature).
func refreshDevices(inv *Inventory) error {
	out, err := execNvidiaSmi(
		"--query-gpu=index,memory.used,memory.free,temperature.gpu,utilization.gpu",
		"--format=csv,noheader,nounits",
	)
	if err != nil {
		return err
	}

	lines := strings.Split(out, "\n")
	devByIdx := make(map[int]*Device)
	for i := range inv.Devices {
		devByIdx[inv.Devices[i].Index] = &inv.Devices[i]
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ", ")
		if len(fields) < 5 {
			continue
		}
		idx, _ := strconv.Atoi(strings.TrimSpace(fields[0]))
		if dev, ok := devByIdx[idx]; ok {
			dev.UsedVRAM, _ = strconv.ParseInt(strings.TrimSpace(fields[1]), 10, 64)
			dev.FreeVRAM, _ = strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
			dev.Temperature, _ = strconv.Atoi(strings.TrimSpace(fields[3]))
			dev.Utilization, _ = strconv.Atoi(strings.TrimSpace(fields[4]))
		}
	}
	inv.Timestamp = time.Now()
	return nil
}

// parseComputeCaps parses nvidia-smi compute_cap output into a map of index -> sm_XX.
func parseComputeCaps(csv string) map[int]string {
	result := make(map[int]string)
	for _, line := range strings.Split(csv, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ", ")
		if len(fields) < 2 {
			continue
		}
		idx, _ := strconv.Atoi(strings.TrimSpace(fields[0]))
		cap := strings.TrimSpace(fields[1])
		// Convert "8.9" -> "sm_89"
		cap = strings.Replace(cap, ".", "", 1)
		result[idx] = "sm_" + cap
	}
	return result
}

// busIDToArch is a fallback that tries to infer architecture from the bus ID.
// Returns empty string if unknown — the compute_cap query is preferred.
func busIDToArch(busID string) string {
	_ = busID
	return ""
}
