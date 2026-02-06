// Package jobs manages the lifecycle of GPU workloads.
// It coordinates model verification, runtime validation, scheduling,
// and isolation backend launching.
package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vornos/buildos/pkg/isolation"
	"github.com/vornos/buildos/pkg/models"
	"github.com/vornos/buildos/pkg/runtime"
	"github.com/vornos/buildos/pkg/scheduler"

	"go.uber.org/zap"
)

// Status tracks a job's lifecycle.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusKilled    Status = "killed"
)

// Job is the core unit of work in VRM.
type Job struct {
	ID          string            `json:"id"`
	ModelID     string            `json:"model_id"`
	RuntimeID   string            `json:"runtime_id"`
	Mode        isolation.Mode    `json:"mode"`
	GPULimit    int               `json:"gpu_limit"`
	VRAMLimit   int64             `json:"vram_limit_mb"`
	CPULimit    int               `json:"cpu_limit"`
	Status      Status            `json:"status"`
	ExitCode    int               `json:"exit_code"`
	Error       string            `json:"error,omitempty"`
	SubmittedAt time.Time         `json:"submitted_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	FinishedAt  *time.Time        `json:"finished_at,omitempty"`
	cancel      context.CancelFunc
}

// Manager orchestrates job execution.
type Manager struct {
	mu           sync.RWMutex
	runtimes     *runtime.Store
	models       *models.Store
	scheduler    *scheduler.Scheduler
	jobs         map[string]*Job
	log          *zap.SugaredLogger
}

// NewManager creates a job manager wired to all subsystems.
func NewManager(
	rt *runtime.Store,
	ms *models.Store,
	sched *scheduler.Scheduler,
	log *zap.SugaredLogger,
) *Manager {
	return &Manager{
		runtimes:  rt,
		models:    ms,
		scheduler: sched,
		jobs:      make(map[string]*Job),
		log:       log,
	}
}

// Submit validates and enqueues a job for execution.
func (m *Manager) Submit(job *Job) error {
	// 1. Verify model signature
	if _, err := m.models.MustVerify(job.ModelID); err != nil {
		return fmt.Errorf("model verification failed: %w", err)
	}

	// 2. Validate runtime layer exists
	if _, err := m.runtimes.Get(job.RuntimeID); err != nil {
		return fmt.Errorf("runtime validation failed: %w", err)
	}

	// 3. Allocate GPUs
	alloc, err := m.scheduler.Allocate(scheduler.Request{
		JobID:    job.ID,
		GPUCount: job.GPULimit,
		VRAMLimit: job.VRAMLimit,
	})
	if err != nil {
		return fmt.Errorf("gpu allocation failed: %w", err)
	}

	job.Status = StatusPending
	job.SubmittedAt = time.Now()

	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	// Launch asynchronously
	go m.execute(job, alloc)

	m.log.Infow("job submitted",
		"job_id", job.ID,
		"model", job.ModelID,
		"runtime", job.RuntimeID,
		"gpus", alloc.DeviceIdxs,
	)
	return nil
}

// execute runs the job in the appropriate isolation backend.
func (m *Manager) execute(job *Job, alloc *scheduler.Allocation) {
	defer m.scheduler.Release(job.ID)

	ctx, cancel := context.WithCancel(context.Background())
	job.cancel = cancel
	defer cancel()

	launcher, err := isolation.NewLauncher(job.Mode)
	if err != nil {
		m.failJob(job, err)
		return
	}

	now := time.Now()
	job.StartedAt = &now
	job.Status = StatusRunning

	m.log.Infow("job started", "job_id", job.ID, "backend", launcher.Name())

	rt, _ := m.runtimes.Get(job.RuntimeID)
	result, err := launcher.Launch(ctx, &isolation.JobSpec{
		JobID:        job.ID,
		RuntimeImage: rt.ImageRef,
		GPUDevices:   alloc.DeviceIdxs,
		VRAMLimit:    job.VRAMLimit,
		CPULimit:     job.CPULimit,
	})
	if err != nil {
		m.failJob(job, err)
		return
	}

	fin := time.Now()
	job.FinishedAt = &fin
	job.ExitCode = result.ExitCode
	if result.ExitCode == 0 {
		job.Status = StatusCompleted
	} else {
		job.Status = StatusFailed
	}
}

func (m *Manager) failJob(job *Job, err error) {
	now := time.Now()
	job.FinishedAt = &now
	job.Status = StatusFailed
	job.Error = err.Error()
	m.log.Errorw("job failed", "job_id", job.ID, "error", err)
}

// Kill cancels a running job.
func (m *Manager) Kill(jobID string) error {
	m.mu.RLock()
	job, ok := m.jobs[jobID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("job %q not found", jobID)
	}
	if job.cancel != nil {
		job.cancel()
	}
	job.Status = StatusKilled
	return nil
}

// List returns all jobs.
func (m *Manager) List() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, j)
	}
	return out
}

// Get returns a job by ID.
func (m *Manager) Get(id string) (*Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job %q not found", id)
	}
	return j, nil
}
