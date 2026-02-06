package store

import (
	"os"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *SQLiteStore {
	t.Helper()
	dbPath := t.TempDir() + "/test.db"
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create SQLite store: %v", err)
	}
	t.Cleanup(func() {
		s.Close()
		os.Remove(dbPath)
	})
	return s
}

func TestSQLiteStore_RuntimeCRUD(t *testing.T) {
	s := setupTestDB(t)

	rt := RuntimeRecord{
		ID:               "rt-001",
		Name:             "pytorch-2.2",
		ImagePath:        "/var/lib/vorn/runtimes/layers/rt-001/image.sif",
		Framework:        "pytorch",
		FrameworkVersion: "2.2.0",
		CUDAVersion:      "12.4",
		SHA256:           "abc123",
		RegisteredAt:     time.Now(),
	}

	if err := s.SaveRuntime(rt); err != nil {
		t.Fatalf("SaveRuntime: %v", err)
	}

	got, err := s.GetRuntime("rt-001")
	if err != nil {
		t.Fatalf("GetRuntime: %v", err)
	}
	if got.Name != "pytorch-2.2" {
		t.Errorf("expected name pytorch-2.2, got %s", got.Name)
	}

	rts, err := s.ListRuntimes()
	if err != nil {
		t.Fatalf("ListRuntimes: %v", err)
	}
	if len(rts) != 1 {
		t.Errorf("expected 1 runtime, got %d", len(rts))
	}

	if err := s.DeleteRuntime("rt-001"); err != nil {
		t.Fatalf("DeleteRuntime: %v", err)
	}

	rts, err = s.ListRuntimes()
	if err != nil {
		t.Fatalf("ListRuntimes after delete: %v", err)
	}
	if len(rts) != 0 {
		t.Errorf("expected 0 runtimes, got %d", len(rts))
	}
}

func TestSQLiteStore_ModelCRUD(t *testing.T) {
	s := setupTestDB(t)

	m := ModelRecord{
		ID:        "model-001",
		Name:      "llama-7b",
		Format:    "safetensors",
		Framework: "pytorch",
		SHA256:    "deadbeef",
		SizeBytes: 7_000_000_000,
		Encrypted: true,
		SignerKey:  "vorn-sign-2025",
		AddedAt:   time.Now(),
	}

	if err := s.SaveModel(m); err != nil {
		t.Fatalf("SaveModel: %v", err)
	}

	got, err := s.GetModel("model-001")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	if got.Name != "llama-7b" {
		t.Errorf("expected name llama-7b, got %s", got.Name)
	}

	models, err := s.ListModels()
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 1 {
		t.Errorf("expected 1 model, got %d", len(models))
	}

	if err := s.DeleteModel("model-001"); err != nil {
		t.Fatalf("DeleteModel: %v", err)
	}
}

func TestSQLiteStore_JobCRUD(t *testing.T) {
	s := setupTestDB(t)

	j := JobRecord{
		ID:          "job-001",
		ModelID:     "model-001",
		RuntimeID:   "rt-001",
		Status:      "running",
		GPUIndices:  "0,1",
		VRAMAllocMB: 16384,
		Mode:        "research",
		SubmittedAt: time.Now(),
		StartedAt:   time.Now(),
	}

	if err := s.SaveJob(j); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}

	got, err := s.GetJob("job-001")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.Status != "running" {
		t.Errorf("expected status running, got %s", got.Status)
	}

	// Update status
	j.Status = "completed"
	j.FinishedAt = time.Now()
	j.ExitCode = 0
	if err := s.SaveJob(j); err != nil {
		t.Fatalf("SaveJob (update): %v", err)
	}

	got, err = s.GetJob("job-001")
	if err != nil {
		t.Fatalf("GetJob after update: %v", err)
	}
	if got.Status != "completed" {
		t.Errorf("expected status completed, got %s", got.Status)
	}

	jobs, err := s.ListJobs("completed")
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("expected 1 completed job, got %d", len(jobs))
	}
}

func TestSQLiteStore_AuditLog(t *testing.T) {
	s := setupTestDB(t)

	entry := AuditEntry{
		Timestamp: time.Now(),
		EventType: "model.add",
		Level:     "info",
		Message:   "Model llama-7b added",
		Details:   `{"model_id":"model-001"}`,
	}

	if err := s.AppendAudit(entry); err != nil {
		t.Fatalf("AppendAudit: %v", err)
	}
}
