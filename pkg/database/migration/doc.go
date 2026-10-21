// Package migration runs ordered, checksummed schema changes.
//
// Run and Rollback acquire a database advisory lock before creating or
// inspecting the migration table. PostgreSQL and MySQL runners pin the lock
// and all migration statements to one physical database/sql connection;
// advisory locks are session-scoped and must not be acquired through one
// pooled connection and used through another.
package migration
