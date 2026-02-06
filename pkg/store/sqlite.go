package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// SQLiteStore implements Store backed by SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens or creates a SQLite database at dbPath.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-64000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

// Migrate runs all embedded SQL migration files.
func (s *SQLiteStore) Migrate(ctx context.Context) error {
	data, err := migrationsFS.ReadFile("migrations/001_initial.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, string(data)); err != nil {
		return fmt.Errorf("exec migration: %w", err)
	}
	return nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// ── Runtimes ──────────────────────────────────────────

func (s *SQLiteStore) SaveRuntime(ctx context.Context, r *RuntimeRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO runtimes (id, name, version, image_ref, image_format, cuda_version, cudnn_version,
			frameworks, architectures, driver_min, sha256, registered_at, validated)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, version=excluded.version, image_ref=excluded.image_ref,
			image_format=excluded.image_format, cuda_version=excluded.cuda_version,
			cudnn_version=excluded.cudnn_version, frameworks=excluded.frameworks,
			architectures=excluded.architectures, driver_min=excluded.driver_min,
			sha256=excluded.sha256, validated=excluded.validated`,
		r.ID, r.Name, r.Version, r.ImageRef, r.ImageFormat, r.CUDAVersion, r.CuDNNVersion,
		r.Frameworks, r.Architectures, r.DriverMin, r.SHA256, r.RegisteredAt, r.Validated,
	)
	return err
}

func (s *SQLiteStore) GetRuntime(ctx context.Context, id string) (*RuntimeRecord, error) {
	r := &RuntimeRecord{}
	err := s.db.QueryRowContext(ctx, `SELECT id, name, version, image_ref, image_format, cuda_version,
		cudnn_version, frameworks, architectures, driver_min, sha256, registered_at, validated
		FROM runtimes WHERE id = ?`, id).Scan(
		&r.ID, &r.Name, &r.Version, &r.ImageRef, &r.ImageFormat, &r.CUDAVersion,
		&r.CuDNNVersion, &r.Frameworks, &r.Architectures, &r.DriverMin, &r.SHA256,
		&r.RegisteredAt, &r.Validated,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("runtime %q not found", id)
	}
	return r, err
}

func (s *SQLiteStore) ListRuntimes(ctx context.Context) ([]*RuntimeRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, version, image_ref, image_format,
		cuda_version, cudnn_version, frameworks, architectures, driver_min, sha256,
		registered_at, validated FROM runtimes ORDER BY registered_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*RuntimeRecord
	for rows.Next() {
		r := &RuntimeRecord{}
		if err := rows.Scan(&r.ID, &r.Name, &r.Version, &r.ImageRef, &r.ImageFormat,
			&r.CUDAVersion, &r.CuDNNVersion, &r.Frameworks, &r.Architectures, &r.DriverMin,
			&r.SHA256, &r.RegisteredAt, &r.Validated); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) DeleteRuntime(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM runtimes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("runtime %q not found", id)
	}
	return nil
}

// ── Models ────────────────────────────────────────────

func (s *SQLiteStore) SaveModel(ctx context.Context, m *ModelRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO models (id, name, version, format, sha256, signature, size_bytes, encrypted, imported_at, verified)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, version=excluded.version, format=excluded.format,
			sha256=excluded.sha256, signature=excluded.signature, size_bytes=excluded.size_bytes,
			encrypted=excluded.encrypted, verified=excluded.verified`,
		m.ID, m.Name, m.Version, m.Format, m.SHA256, m.Signature, m.SizeBytes,
		m.Encrypted, m.ImportedAt, m.Verified,
	)
	return err
}

func (s *SQLiteStore) GetModel(ctx context.Context, id string) (*ModelRecord, error) {
	m := &ModelRecord{}
	err := s.db.QueryRowContext(ctx, `SELECT id, name, version, format, sha256, signature,
		size_bytes, encrypted, imported_at, verified FROM models WHERE id = ?`, id).Scan(
		&m.ID, &m.Name, &m.Version, &m.Format, &m.SHA256, &m.Signature,
		&m.SizeBytes, &m.Encrypted, &m.ImportedAt, &m.Verified,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("model %q not found", id)
	}
	return m, err
}

func (s *SQLiteStore) ListModels(ctx context.Context) ([]*ModelRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, version, format, sha256, signature,
		size_bytes, encrypted, imported_at, verified FROM models ORDER BY imported_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*ModelRecord
	for rows.Next() {
		m := &ModelRecord{}
		if err := rows.Scan(&m.ID, &m.Name, &m.Version, &m.Format, &m.SHA256, &m.Signature,
			&m.SizeBytes, &m.Encrypted, &m.ImportedAt, &m.Verified); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) DeleteModel(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM models WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("model %q not found", id)
	}
	return nil
}

// ── Jobs ──────────────────────────────────────────────

func (s *SQLiteStore) SaveJob(ctx context.Context, j *JobRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO jobs (id, model_id, runtime_id, mode, gpu_limit, vram_limit_mb, cpu_limit,
			status, exit_code, error, submitted_at, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status=excluded.status, exit_code=excluded.exit_code, error=excluded.error,
			started_at=excluded.started_at, finished_at=excluded.finished_at`,
		j.ID, j.ModelID, j.RuntimeID, j.Mode, j.GPULimit, j.VRAMLimit, j.CPULimit,
		j.Status, j.ExitCode, j.Error, j.SubmittedAt, j.StartedAt, j.FinishedAt,
	)
	return err
}

func (s *SQLiteStore) GetJob(ctx context.Context, id string) (*JobRecord, error) {
	j := &JobRecord{}
	err := s.db.QueryRowContext(ctx, `SELECT id, model_id, runtime_id, mode, gpu_limit,
		vram_limit_mb, cpu_limit, status, exit_code, error, submitted_at, started_at, finished_at
		FROM jobs WHERE id = ?`, id).Scan(
		&j.ID, &j.ModelID, &j.RuntimeID, &j.Mode, &j.GPULimit,
		&j.VRAMLimit, &j.CPULimit, &j.Status, &j.ExitCode, &j.Error,
		&j.SubmittedAt, &j.StartedAt, &j.FinishedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job %q not found", id)
	}
	return j, err
}

func (s *SQLiteStore) ListJobs(ctx context.Context, filter JobFilter) ([]*JobRecord, error) {
	query := `SELECT id, model_id, runtime_id, mode, gpu_limit, vram_limit_mb, cpu_limit,
		status, exit_code, error, submitted_at, started_at, finished_at FROM jobs WHERE 1=1`
	var args []interface{}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.ModelID != "" {
		query += " AND model_id = ?"
		args = append(args, filter.ModelID)
	}
	query += " ORDER BY submitted_at DESC"
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*JobRecord
	for rows.Next() {
		j := &JobRecord{}
		if err := rows.Scan(&j.ID, &j.ModelID, &j.RuntimeID, &j.Mode, &j.GPULimit,
			&j.VRAMLimit, &j.CPULimit, &j.Status, &j.ExitCode, &j.Error,
			&j.SubmittedAt, &j.StartedAt, &j.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) UpdateJobStatus(ctx context.Context, id, status string, exitCode int, errMsg string) error {
	now := time.Now()
	var query string
	var args []interface{}

	switch status {
	case "running":
		query = `UPDATE jobs SET status = ?, started_at = ? WHERE id = ?`
		args = []interface{}{status, now, id}
	case "completed", "failed", "killed":
		query = `UPDATE jobs SET status = ?, exit_code = ?, error = ?, finished_at = ? WHERE id = ?`
		args = []interface{}{status, exitCode, errMsg, now, id}
	default:
		query = `UPDATE jobs SET status = ? WHERE id = ?`
		args = []interface{}{status, id}
	}

	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("job %q not found", id)
	}
	return nil
}

// ── Scheduler State ───────────────────────────────────

func (s *SQLiteStore) SaveSchedulerState(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO scheduler_state (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		key, value, time.Now(),
	)
	return err
}

func (s *SQLiteStore) LoadSchedulerState(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM scheduler_state WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// ── Audit Log ─────────────────────────────────────────

func (s *SQLiteStore) AppendAudit(ctx context.Context, entry *AuditEntry) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_log (timestamp, event, actor, resource, result, detail)
		VALUES (?, ?, ?, ?, ?, ?)`,
		entry.Timestamp, entry.Event, entry.Actor, entry.Resource, entry.Result, entry.Detail,
	)
	return err
}

func (s *SQLiteStore) QueryAudit(ctx context.Context, filter AuditFilter) ([]*AuditEntry, error) {
	query := `SELECT id, timestamp, event, actor, resource, result, detail FROM audit_log WHERE 1=1`
	var args []interface{}

	if filter.Event != "" {
		query += " AND event = ?"
		args = append(args, filter.Event)
	}
	if !filter.After.IsZero() {
		query += " AND timestamp > ?"
		args = append(args, filter.After)
	}
	query += " ORDER BY timestamp DESC"
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*AuditEntry
	for rows.Next() {
		e := &AuditEntry{}
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Event, &e.Actor, &e.Resource,
			&e.Result, &e.Detail); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
