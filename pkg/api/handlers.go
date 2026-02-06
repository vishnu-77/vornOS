package api

import (
	"context"
	"fmt"
	"time"

	"github.com/vornos/buildos/pkg/config"
	"github.com/vornos/buildos/pkg/gpu"
	"github.com/vornos/buildos/pkg/jobs"
	"github.com/vornos/buildos/pkg/logging"
	"github.com/vornos/buildos/pkg/models"
	"github.com/vornos/buildos/pkg/runtime"
	"github.com/vornos/buildos/pkg/scheduler"
	"github.com/vornos/buildos/pkg/store"

	"go.uber.org/zap"
)

// Handlers implements all VornService RPCs.
type Handlers struct {
	cfg          *config.Config
	store        store.Store
	runtimes     *runtime.Store
	models       *models.Store
	jobManager   *jobs.Manager
	scheduler    *scheduler.Scheduler
	gpuInventory *gpu.Inventory
	audit        *logging.AuditLogger
	log          *zap.SugaredLogger
	startTime    time.Time
}

// NewHandlers creates the RPC handler set wired to all subsystems.
func NewHandlers(deps *Deps) *Handlers {
	return &Handlers{
		cfg:          deps.Config,
		store:        deps.Store,
		runtimes:     deps.RuntimeStore,
		models:       deps.ModelStore,
		jobManager:   deps.JobManager,
		scheduler:    deps.Scheduler,
		gpuInventory: deps.GPUInventory,
		audit:        deps.AuditLogger,
		log:          deps.Logger,
		startTime:    time.Now(),
	}
}

// ── Status ────────────────────────────────────────────

// GetStatus returns daemon status and summary metrics.
func (h *Handlers) GetStatus(ctx context.Context) (map[string]interface{}, error) {
	gpuCount := 0
	if h.gpuInventory != nil {
		gpuCount = len(h.gpuInventory.Devices)
	}

	allJobs := h.jobManager.List()
	active, pending := 0, 0
	for _, j := range allJobs {
		switch j.Status {
		case jobs.StatusRunning:
			active++
		case jobs.StatusPending:
			pending++
		}
	}

	return map[string]interface{}{
		"version":        config.Version,
		"uptime_seconds": int64(time.Since(h.startTime).Seconds()),
		"gpu_count":      gpuCount,
		"active_jobs":    active,
		"pending_jobs":   pending,
		"runtimes":       len(h.runtimes.List()),
		"models":         len(h.models.List()),
	}, nil
}

// ── Runtimes ──────────────────────────────────────────

// RegisterRuntime adds a new runtime layer.
func (h *Handlers) RegisterRuntime(ctx context.Context, name, version, imageRef, imageFormat,
	cudaVersion, cudnnVersion string, frameworks, architectures []string, driverMin string,
) (string, error) {
	layer := &runtime.Layer{
		Name:          name,
		Version:       version,
		ImageRef:      imageRef,
		ImageFormat:   imageFormat,
		CUDAVersion:   cudaVersion,
		CuDNNVersion:  cudnnVersion,
		Frameworks:    frameworks,
		Architectures: architectures,
		DriverMin:     driverMin,
	}

	if err := h.runtimes.Register(layer, imageRef); err != nil {
		return "", fmt.Errorf("register runtime: %w", err)
	}

	h.audit.Log(ctx, logging.AuditRuntimeRegister, "api", layer.ID, "success", nil)
	h.log.Infow("runtime registered", "id", layer.ID, "name", name, "cuda", cudaVersion)
	return layer.ID, nil
}

// ── Models ────────────────────────────────────────────

// VerifyModel checks a model's signature.
func (h *Handlers) VerifyModel(ctx context.Context, id string) (bool, string, error) {
	err := h.models.Verify(id)
	if err != nil {
		h.audit.Log(ctx, logging.AuditModelVerifyFail, "api", id, "failure",
			map[string]interface{}{"error": err.Error()})
		return false, "", err
	}

	h.audit.Log(ctx, logging.AuditModelVerify, "api", id, "success", nil)
	return true, "", nil
}

// ── Jobs ──────────────────────────────────────────────

// SubmitJob creates and enqueues a job.
func (h *Handlers) SubmitJob(ctx context.Context, modelID, runtimeID, mode string,
	gpuLimit int, vramLimit int64, cpuLimit int,
) (string, error) {
	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	job := &jobs.Job{
		ID:        jobID,
		ModelID:   modelID,
		RuntimeID: runtimeID,
		GPULimit:  gpuLimit,
		VRAMLimit: vramLimit,
		CPULimit:  cpuLimit,
	}

	// Parse isolation mode
	switch mode {
	case "appliance", "research", "secure":
		// valid
	default:
		mode = h.cfg.DefaultIsolation
	}

	if err := h.jobManager.Submit(job); err != nil {
		h.audit.Log(ctx, logging.AuditJobFail, "api", jobID, "failure",
			map[string]interface{}{"error": err.Error()})
		return "", err
	}

	h.audit.Log(ctx, logging.AuditJobSubmit, "api", jobID, "success",
		map[string]interface{}{"model": modelID, "runtime": runtimeID, "mode": mode})
	return jobID, nil
}

// KillJob cancels a running job.
func (h *Handlers) KillJob(ctx context.Context, jobID string) error {
	if err := h.jobManager.Kill(jobID); err != nil {
		return err
	}
	h.audit.Log(ctx, logging.AuditJobKill, "api", jobID, "success", nil)
	return nil
}
