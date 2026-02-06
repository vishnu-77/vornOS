// Package gpu handles discovery and querying of NVIDIA GPU devices.
// It wraps nvidia-smi output into structured inventory data.
package gpu

import (
	"time"
)

// Device represents a single GPU.
type Device struct {
	Index       int    `json:"index"`
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	DriverVer   string `json:"driver_version"`
	CUDAVer     string `json:"cuda_version"`
	TotalVRAM   int64  `json:"total_vram_mb"`   // megabytes
	UsedVRAM    int64  `json:"used_vram_mb"`
	FreeVRAM    int64  `json:"free_vram_mb"`
	Temperature int    `json:"temperature_c"`
	Utilization int    `json:"utilization_pct"` // 0-100
	Architecture string `json:"architecture"`   // e.g. "sm_80"
}

// Inventory is the result of GPU discovery.
type Inventory struct {
	Devices   []Device  `json:"devices"`
	Timestamp time.Time `json:"timestamp"`
}

// Discover probes the system for NVIDIA GPUs via nvidia-smi.
// Returns an Inventory or an error if no GPUs / nvidia-smi not found.
func Discover() (*Inventory, error) {
	return discoverDevices()
}

// Refresh updates VRAM usage and utilization for all devices in the inventory.
func (inv *Inventory) Refresh() error {
	return refreshDevices(inv)
}

// DeviceByIndex returns the device at the given index, or nil if not found.
func (inv *Inventory) DeviceByIndex(idx int) *Device {
	for i := range inv.Devices {
		if inv.Devices[i].Index == idx {
			return &inv.Devices[i]
		}
	}
	return nil
}

// DriverVersion returns the driver version from the first device,
// or empty string if no devices exist.
func (inv *Inventory) DriverVersion() string {
	if len(inv.Devices) == 0 {
		return ""
	}
	return inv.Devices[0].DriverVer
}

// TotalVRAM returns the aggregate VRAM across all devices.
func (inv *Inventory) TotalVRAM() int64 {
	var total int64
	for _, d := range inv.Devices {
		total += d.TotalVRAM
	}
	return total
}

// Stale returns true if the inventory is older than the given duration.
func (inv *Inventory) Stale(maxAge time.Duration) bool {
	return time.Since(inv.Timestamp) > maxAge
}
