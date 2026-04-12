package database

// TableNamer allows models to specify a custom table name.
type TableNamer interface {
	TableName() string
}
