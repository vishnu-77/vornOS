package gpu

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// HealthState represents the health of a single GPU.
type HealthState string

const (
	HealthHealthy  HealthState = "healthy"
	HealthDegraded HealthState = "degraded"
	HealthFailed   HealthState = "failed"
)

// GPUHealth tracks per-device health status.
type GPUHealth struct {
	DeviceIndex   int         `json:"device_index"`
	State         HealthState `json:"state"`
	Temperature   int         `json:"temperature_c"`
	ECCCorrected  int64       `json:"ecc_corrected"`
	ECCUncorrected int64      `json:"ecc_uncorrected"`
	LastError     string      `json:"last_error,omitempty"`
	LastChecked   time.Time   `json:"last_checked"`
}

// HealthMonitor polls nvidia-smi for GPU health metrics and detects faults.
type HealthMonitor struct {
	mu           sync.RWMutex
	inventory    *Inventory
	health       map[int]*GPUHealth // keyed by device index
	pollInterval time.Duration
	onEvent      func(event HealthEvent) // callback for health events
}

// HealthEvent is emitted when a GPU's health state changes.
type HealthEvent struct {
	DeviceIndex int         `json:"device_index"`
	OldState    HealthState `json:"old_state"`
	NewState    HealthState `json:"new_state"`
	Reason      string      `json:"reason"`
	Timestamp   time.Time   `json:"timestamp"`
}

// NewHealthMonitor creates a monitor for the given inventory.
func NewHealthMonitor(inv *Inventory, pollInterval time.Duration, onEvent func(HealthEvent)) *HealthMonitor {
	m := &HealthMonitor{
		inventory:    inv,
		health:       make(map[int]*GPUHealth),
		pollInterval: pollInterval,
		onEvent:      onEvent,
	}

	// Initialize health for all known devices
	for _, d := range inv.Devices {
		m.health[d.Index] = &GPUHealth{
			DeviceIndex: d.Index,
			State:       HealthHealthy,
			LastChecked: time.Now(),
		}
	}

	return m
}

// Start begins the health polling loop. Blocks until ctx is cancelled.
func (m *HealthMonitor) Start(ctx context.Context) error {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			m.poll()
		}
	}
}

// GetHealth returns the current health of a specific device.
func (m *HealthMonitor) GetHealth(deviceIdx int) *GPUHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.health[deviceIdx]
}

// AllHealthy returns true if all GPUs are in healthy state.
func (m *HealthMonitor) AllHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, h := range m.health {
		if h.State != HealthHealthy {
			return false
		}
	}
	return true
}

// poll queries nvidia-smi for health metrics and updates state.
func (m *HealthMonitor) poll() {
	// Query ECC errors and temperature
	out, err := execNvidiaSmi(
		"--query-gpu=index,temperature.gpu,ecc.errors.corrected.volatile.total,ecc.errors.uncorrected.volatile.total",
		"--format=csv,noheader,nounits",
	)
	if err != nil {
		// If nvidia-smi fails entirely, mark all GPUs as failed
		m.mu.Lock()
		for idx, h := range m.health {
			if h.State != HealthFailed {
				old := h.State
				h.State = HealthFailed
				h.LastError = fmt.Sprintf("nvidia-smi unreachable: %v", err)
				h.LastChecked = time.Now()
				if m.onEvent != nil {
					m.onEvent(HealthEvent{
						DeviceIndex: idx,
						OldState:    old,
						NewState:    HealthFailed,
						Reason:      h.LastError,
						Timestamp:   time.Now(),
					})
				}
			}
		}
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ", ")
		if len(fields) < 4 {
			continue
		}

		idx, _ := strconv.Atoi(strings.TrimSpace(fields[0]))
		temp, _ := strconv.Atoi(strings.TrimSpace(fields[1]))
		eccCorr, _ := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
		eccUncorr, _ := strconv.ParseInt(strings.TrimSpace(fields[3]), 10, 64)

		h, ok := m.health[idx]
		if !ok {
			continue
		}

		oldState := h.State
		h.Temperature = temp
		h.ECCCorrected = eccCorr
		h.ECCUncorrected = eccUncorr
		h.LastChecked = time.Now()

		// Fault detection rules
		newState := HealthHealthy
		reason := ""

		if eccUncorr > 0 {
			newState = HealthFailed
			reason = fmt.Sprintf("uncorrectable ECC errors: %d", eccUncorr)
		} else if temp > 90 {
			newState = HealthFailed
			reason = fmt.Sprintf("critical temperature: %dC", temp)
		} else if temp > 83 {
			newState = HealthDegraded
			reason = fmt.Sprintf("high temperature: %dC", temp)
		} else if eccCorr > 100 {
			newState = HealthDegraded
			reason = fmt.Sprintf("excessive correctable ECC errors: %d", eccCorr)
		}

		if newState != oldState {
			h.State = newState
			h.LastError = reason
			if m.onEvent != nil {
				m.onEvent(HealthEvent{
					DeviceIndex: idx,
					OldState:    oldState,
					NewState:    newState,
					Reason:      reason,
					Timestamp:   time.Now(),
				})
			}
		}
	}
}
