package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vornos/buildos/pkg/store"
)

// AuditLogger writes security-relevant events to both a JSON file and the database.
type AuditLogger struct {
	mu       sync.Mutex
	file     *os.File
	db       store.Store
	filePath string
}

// NewAuditLogger creates an audit logger. If db is nil, only file logging is used.
func NewAuditLogger(logDir string, db store.Store) (*AuditLogger, error) {
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return nil, fmt.Errorf("create audit log dir: %w", err)
	}

	filePath := filepath.Join(logDir, "audit.log")
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return nil, fmt.Errorf("open audit log: %w", err)
	}

	return &AuditLogger{
		file:     f,
		db:       db,
		filePath: filePath,
	}, nil
}

// Log writes an audit entry to both the file and the database.
func (a *AuditLogger) Log(ctx context.Context, event, actor, resource, result string, detail map[string]interface{}) {
	entry := &store.AuditEntry{
		Timestamp: time.Now().UTC(),
		Event:     event,
		Actor:     actor,
		Resource:  resource,
		Result:    result,
	}

	if detail != nil {
		detailJSON, _ := json.Marshal(detail)
		entry.Detail = string(detailJSON)
	}

	// Write to file (JSON lines)
	a.mu.Lock()
	line := map[string]interface{}{
		"timestamp": entry.Timestamp.Format(time.RFC3339Nano),
		"event":     event,
		"actor":     actor,
		"resource":  resource,
		"result":    result,
		"detail":    detail,
	}
	data, _ := json.Marshal(line)
	data = append(data, '\n')
	a.file.Write(data)
	a.mu.Unlock()

	// Write to database
	if a.db != nil {
		a.db.AppendAudit(ctx, entry)
	}
}

// Close flushes and closes the audit log file.
func (a *AuditLogger) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.file.Close()
}
