package database

// TableNamer lets a persisted Go type specify a custom table name.
type TableNamer interface {
	TableName() string
}
