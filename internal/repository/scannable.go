package repository

// scannable is any row-like value that can be scanned into destination
// pointers — satisfied by both *sql.Row and *sql.Rows. Used by the scan
// helpers in the new-schema repositories.
type scannable interface {
	Scan(dest ...any) error
}

// boolToInt converts a Go bool into the 0/1 SQLite convention.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
