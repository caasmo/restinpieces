package db

// LogDB defines the interface for database operations related to logs.
type DbLog interface {
	// InsertBatch inserts a batch of log entries into the database.
	InsertBatch(batch []Log) error
	// Close closes the underlying database connection or pool.
	Close() error
}
