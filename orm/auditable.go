package orm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/astraframework/astra/json"
)

// Auditable is a mixin that enables automatic audit logging.
// Embed this in your models to track changes automatically.
type Auditable struct{}

var (
	auditMutex  sync.Mutex
	auditLogger *AuditLogger
)

// AuditLogger handles audit log writing
type AuditLogger struct {
	file *os.File
	mu   sync.Mutex
}

// ActorKey is the context key for the current actor
type ActorKey struct{}

// Actor represents the user performing an action
type Actor struct {
	ID string
	IP string
}

// InitializeAuditLogger handles setting up the audit log file.
// This should be called during application boot.
func InitializeAuditLogger(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create audit directory %s: %w", dir, err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit log %s: %w", path, err)
	}

	auditMutex.Lock()
	defer auditMutex.Unlock()
	auditLogger = &AuditLogger{file: f}
	return nil
}

// AuditEntry represents a single audit log entry
type AuditEntry struct {
	Timestamp string         `json:"timestamp"`
	Action    string         `json:"action"`
	Model     string         `json:"model"`
	RecordID  any            `json:"record_id"`
	Actor     *Actor         `json:"actor,omitempty"`
	Changes   map[string]any `json:"changes,omitempty"`
}

func (al *AuditLogger) Log(entry AuditEntry) error {
	if al == nil || al.file == nil {
		return nil // Audit logging disabled or not initialized
	}
	al.mu.Lock()
	defer al.mu.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = al.file.Write(append(data, '\n'))
	return err
}

// Hook implementations for Auditable (integrated with orm.hooks)

func (a Auditable) AfterCreate(ctx context.Context, db *DB, model any) error {
	actor, _ := ctx.Value(ActorKey{}).(*Actor)
	meta := GetMeta(reflect.TypeOf(model))

	entry := AuditEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Action:    "created",
		Model:     meta.TableName,
		RecordID:  fieldByIndex(reflect.ValueOf(model).Elem(), meta.PK.FieldIndex).Interface(),
		Actor:     actor,
	}

	return auditLogger.Log(entry)
}

func (a Auditable) AfterUpdate(ctx context.Context, db *DB, model any) error {
	actor, _ := ctx.Value(ActorKey{}).(*Actor)
	meta := GetMeta(reflect.TypeOf(model))

	entry := AuditEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Action:    "updated",
		Model:     meta.TableName,
		RecordID:  fieldByIndex(reflect.ValueOf(model).Elem(), meta.PK.FieldIndex).Interface(),
		Actor:     actor,
		// In a more advanced implementation, we would extract changed fields from the context or model
	}

	return auditLogger.Log(entry)
}

// CloseAuditLog closes the audit log file
func CloseAuditLog() error {
	if auditLogger != nil && auditLogger.file != nil {
		return auditLogger.file.Close()
	}
	return nil
}
