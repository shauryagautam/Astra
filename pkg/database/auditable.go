package database

import "context"

// AuditEntry represents a single change record in the audit log.
type AuditEntry struct {
	Action    string
	Table     string
	RecordID  string
	UserID    string
	Changes   map[string]any
	Timestamp int64
}

// Auditable is an interface for models that support auditing.
type Auditable interface {
	AfterCreate(ctx context.Context, db *DB, model any) error
	AfterUpdate(ctx context.Context, db *DB, model any) error
}
