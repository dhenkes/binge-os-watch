package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// seedUserForMedia creates a fresh user row for a test and returns its id.
// Inherits its name from pre-Option-B helpers but no longer creates any
// media — those are now per-show/movie catalog rows.
func seedUserForMedia(t *testing.T, db DBTX) string {
	t.Helper()
	id := uuid.NewString()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO users (id, username, password, role, created_at)
		 VALUES (?, ?, ?, 'user', strftime('%s','now'))`,
		id, "user_"+id[:8], "hash")
	if err != nil {
		t.Fatalf("seedUserForMedia: %v", err)
	}
	return id
}

// seedMedia is a back-compat shim used by a few legacy tests. Returns
// (userID, "") since there is no per-user media id under the new schema.
func seedMedia(t *testing.T, db DBTX) (string, string) {
	t.Helper()
	return seedUserForMedia(t, db), ""
}
