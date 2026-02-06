// Package store provides a persistence interface and SQLite implementation
// for all Vorn metadata: runtimes, models, jobs, scheduler state, and audit logs.
package store

import (
	"context"
	"time"
)

// RuntimeRecord is the persistence representation of a runtime layer.
type RuntimeRecord struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Version       string    `json:"version"`
	ImageRef      string    `json:"image_ref"`
	ImageFormat   string    `json:"image_format"`
	CUDAVersion   string    `json:"cuda_version"`
	CuDNNVersion  string    `json:"cudnn_version"`
	Frameworks    string    `json:"frameworks"`     // JSON array
	Architectures string    `json:"architectures"`  // JSON array
	DriverMin     string    `json:"driver_min"`
	SHA256        string    `json:"sha256"`
	RegisteredAt  time.Time `json:"registered_at"`
	Validated     bool      `json:"validated"`
}

// ModelRecord is the persistence representation of a model artifact.
type ModelRecord struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Version    string    `json:"version"`
	Format     string    `json:"format"`
	SHA256     string    `json:"sha256"`
	Signature  []byte    `json:"signature"`
	SizeBytes  int64     `json:"size_bytes"`
	Encrypted  bool      `json:"encrypted"`
	ImportedAt time.Time `json:"imported_at"`
	Verified   bool      `json:"verified"`
}

// JobRecord is the persistence representation of a job.
type JobRecord struct {
	ID          string     `json:"id"`
	ModelID     string     `json:"model_id"`
	RuntimeID   string     `json:"runtime_id"`
	Mode        string     `json:"mode"`
	GPULimit    int        `json:"gpu_limit"`
	VRAMLimit   int64      `json:"vram_limit_mb"`
	CPULimit    int        `json:"cpu_limit"`
	Status      string     `json:"status"`
	ExitCode    int        `json:"exit_code"`
	Error       string     `json:"error"`
	SubmittedAt time.Time  `json:"submitted_at"`
	StartedAt   *time.Time `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
}

// AuditEntry represents a security-relevant event.
type AuditEntry struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Event     string    `json:"event"`
	Actor     string    `json:"actor"`
	Resource  string    `json:"resource"`
	Result    string    `json:"result"`
	Detail    string    `json:"detail"` // JSON blob for extra context
}

// JobFilter selects jobs for listing.
type JobFilter struct {
	Status  string // empty = all
	ModelID string // empty = all
	Limit   int    // 0 = no limit
}

// AuditFilter selects audit entries.
type AuditFilter struct {
	Event string
	After time.Time
	Limit int
}

// Store is the persistence interface for all Vorn metadata.
type Store interface {
	// Runtimes
	SaveRuntime(ctx context.Context, r *RuntimeRecord) error
	GetRuntime(ctx context.Context, id string) (*RuntimeRecord, error)
	ListRuntimes(ctx context.Context) ([]*RuntimeRecord, error)
	DeleteRuntime(ctx context.Context, id string) error

	// Models
	SaveModel(ctx context.Context, m *ModelRecord) error
	GetModel(ctx context.Context, id string) (*ModelRecord, error)
	ListModels(ctx context.Context) ([]*ModelRecord, error)
	DeleteModel(ctx context.Context, id string) error

	// Jobs
	SaveJob(ctx context.Context, j *JobRecord) error
	GetJob(ctx context.Context, id string) (*JobRecord, error)
	ListJobs(ctx context.Context, filter JobFilter) ([]*JobRecord, error)
	UpdateJobStatus(ctx context.Context, id, status string, exitCode int, errMsg string) error

	// Scheduler state
	SaveSchedulerState(ctx context.Context, key, value string) error
	LoadSchedulerState(ctx context.Context, key string) (string, error)

	// Audit
	AppendAudit(ctx context.Context, entry *AuditEntry) error
	QueryAudit(ctx context.Context, filter AuditFilter) ([]*AuditEntry, error)

	// Lifecycle
	Migrate(ctx context.Context) error
	Close() error
}
