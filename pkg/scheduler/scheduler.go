// Package scheduler implements GPU-aware job placement.
// MVP uses a simple bin-packing heuristic over available VRAM.
package scheduler

import (
	"fmt"
	"sync"

	"github.com/vornos/buildos/pkg/gpu"
)

// Allocation represents GPUs assigned to a job.
type Allocation struct {
	JobID      string
	DeviceIdxs []int   // indices into gpu.Inventory.Devices
	VRAMPerGPU int64   // MB reserved per GPU
}

// Scheduler assigns GPUs to jobs based on available resources.
type Scheduler struct {
	mu          sync.Mutex
	inventory   *gpu.Inventory
	allocations map[string]*Allocation // jobID -> allocation
}

// New creates a scheduler backed by the given GPU inventory.
func New(inv *gpu.Inventory) *Scheduler {
	return &Scheduler{
		inventory:   inv,
		allocations: make(map[string]*Allocation),
	}
}

// Request describes a job's GPU requirements.
type Request struct {
	JobID     string
	GPUCount  int   // number of GPUs needed
	VRAMLimit int64 // VRAM per GPU in MB (0 = no limit)
}

// Allocate tries to find GPUs satisfying the request.
// Uses a first-fit packing strategy: pick GPUs with the most free VRAM first.
func (s *Scheduler) Allocate(req Request) (*Allocation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.inventory == nil || len(s.inventory.Devices) == 0 {
		return nil, fmt.Errorf("no GPUs available")
	}

	// Calculate available VRAM per device (total - already allocated)
	available := s.availableVRAM()

	// Find GPUs that satisfy the VRAM requirement
	var picked []int
	for idx, freeVRAM := range available {
		if req.VRAMLimit > 0 && freeVRAM < req.VRAMLimit {
			continue
		}
		picked = append(picked, idx)
		if len(picked) == req.GPUCount {
			break
		}
	}

	if len(picked) < req.GPUCount {
		return nil, fmt.Errorf("insufficient GPUs: need %d with >=%d MB VRAM, found %d",
			req.GPUCount, req.VRAMLimit, len(picked))
	}

	alloc := &Allocation{
		JobID:      req.JobID,
		DeviceIdxs: picked,
		VRAMPerGPU: req.VRAMLimit,
	}
	s.allocations[req.JobID] = alloc
	return alloc, nil
}

// Release frees GPUs allocated to a job.
func (s *Scheduler) Release(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.allocations, jobID)
}

// availableVRAM returns free VRAM per device index, accounting for allocations.
func (s *Scheduler) availableVRAM() map[int]int64 {
	free := make(map[int]int64)
	for _, d := range s.inventory.Devices {
		free[d.Index] = d.TotalVRAM
	}
	for _, alloc := range s.allocations {
		for _, idx := range alloc.DeviceIdxs {
			free[idx] -= alloc.VRAMPerGPU
		}
	}
	return free
}
