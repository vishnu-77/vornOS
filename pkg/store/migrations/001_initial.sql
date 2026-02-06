-- Vorn OS initial schema

CREATE TABLE IF NOT EXISTS runtimes (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    version       TEXT NOT NULL,
    image_ref     TEXT NOT NULL,
    image_format  TEXT NOT NULL CHECK(image_format IN ('sif', 'oci')),
    cuda_version  TEXT NOT NULL,
    cudnn_version TEXT,
    frameworks    TEXT DEFAULT '[]',
    architectures TEXT DEFAULT '[]',
    driver_min    TEXT,
    sha256        TEXT NOT NULL,
    registered_at DATETIME NOT NULL,
    validated     BOOLEAN DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS models (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    version     TEXT,
    format      TEXT NOT NULL,
    sha256      TEXT NOT NULL,
    signature   BLOB NOT NULL,
    size_bytes  INTEGER NOT NULL,
    encrypted   BOOLEAN DEFAULT TRUE,
    imported_at DATETIME NOT NULL,
    verified    BOOLEAN DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS jobs (
    id           TEXT PRIMARY KEY,
    model_id     TEXT NOT NULL REFERENCES models(id),
    runtime_id   TEXT NOT NULL REFERENCES runtimes(id),
    mode         TEXT NOT NULL CHECK(mode IN ('appliance', 'research', 'secure')),
    gpu_limit    INTEGER NOT NULL DEFAULT 1,
    vram_limit_mb INTEGER NOT NULL DEFAULT 0,
    cpu_limit    INTEGER DEFAULT 0,
    status       TEXT NOT NULL CHECK(status IN ('pending', 'running', 'completed', 'failed', 'killed')),
    exit_code    INTEGER DEFAULT 0,
    error        TEXT,
    submitted_at DATETIME NOT NULL,
    started_at   DATETIME,
    finished_at  DATETIME
);

CREATE TABLE IF NOT EXISTS scheduler_state (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS audit_log (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    event     TEXT NOT NULL,
    actor     TEXT,
    resource  TEXT,
    result    TEXT NOT NULL,
    detail    TEXT
);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_submitted ON jobs(submitted_at);
CREATE INDEX IF NOT EXISTS idx_jobs_model ON jobs(model_id);
CREATE INDEX IF NOT EXISTS idx_audit_event ON audit_log(event);
CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp);
