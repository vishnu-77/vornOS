package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthStatus represents the daemon health check response.
type HealthStatus struct {
	Status             string `json:"status"` // healthy | degraded | unhealthy
	Version            string `json:"version"`
	UptimeSeconds      int64  `json:"uptime_seconds"`
	GPUCount           int    `json:"gpu_count"`
	GPUHealthy         int    `json:"gpu_healthy"`
	ActiveJobs         int    `json:"active_jobs"`
	ModelStoreUnlocked bool   `json:"model_store_unlocked"`
	DBResponsive       bool   `json:"db_responsive"`
}

// RegisterHealthHandlers adds /healthz and /readyz to the given mux.
func (h *Handlers) RegisterHealthHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", h.handleHealthz)
	mux.HandleFunc("/readyz", h.handleReadyz)
}

// handleHealthz is the liveness probe: is the process alive and not deadlocked?
func (h *Handlers) handleHealthz(w http.ResponseWriter, r *http.Request) {
	gpuCount := 0
	if h.gpuInventory != nil {
		gpuCount = len(h.gpuInventory.Devices)
	}

	status := HealthStatus{
		Status:        "healthy",
		Version:       h.cfg.Version,
		UptimeSeconds: int64(time.Since(h.startTime).Seconds()),
		GPUCount:      gpuCount,
		GPUHealthy:    gpuCount, // TODO: check per-GPU health state
		DBResponsive:  h.store != nil,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// handleReadyz is the readiness probe: is the system fully initialized?
func (h *Handlers) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ready := true
	reasons := []string{}

	if h.gpuInventory == nil || len(h.gpuInventory.Devices) == 0 {
		ready = false
		reasons = append(reasons, "gpu_discovery_pending")
	}

	if h.store == nil {
		ready = false
		reasons = append(reasons, "db_not_initialized")
	}

	if ready {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ready": true}`))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		resp := map[string]interface{}{"ready": false, "reasons": reasons}
		json.NewEncoder(w).Encode(resp)
	}
}
