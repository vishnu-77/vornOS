package store

// Migration files are embedded via //go:embed in sqlite.go.
// Add new migration files to the migrations/ directory as:
//   002_audit_log.sql
//   003_add_column.sql
// etc.
//
// The Migrate() method on SQLiteStore executes them in order.
// For the MVP we run all migrations on every startup (CREATE IF NOT EXISTS is idempotent).
// A production system should track applied migrations in a schema_version table.
