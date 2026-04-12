package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
)

// testDB opens an in-memory SQLite database with migrations applied.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	name := fmt.Sprintf("file:test_%p?mode=memory&cache=shared", t)
	db, err := NewSQLiteDB(name)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestNewSQLiteDB_InMemory(t *testing.T) {
	db := testDB(t)

	// Verify migrations ran: schema_migrations should exist.
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("querying schema_migrations: %v", err)
	}
	if count == 0 {
		t.Error("expected at least 1 migration applied")
	}
}

func TestNewSQLiteDB_TablesExist(t *testing.T) {
	db := testDB(t)

	tables := []string{
		"users", "user_settings", "sessions",
		"tmdb_show", "tmdb_movie", "tmdb_season", "tmdb_episode",
		"user_library", "watch_event",
		"rating_show", "rating_movie", "rating_season", "rating_episode",
		"tag", "library_tag",
		"keyword_watches", "keyword_results", "webhooks", "webhook_deliveries",
	}
	for _, table := range tables {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q: %v", table, err)
		}
	}
}

func TestNewSQLiteDB_ForeignKeysEnabled(t *testing.T) {
	db := testDB(t)

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}

func TestNewSQLiteDB_WALMode(t *testing.T) {
	db := testDB(t)

	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if mode != "wal" && mode != "memory" {
		t.Errorf("journal_mode = %q, want wal or memory", mode)
	}
}

func TestNewTxFunc(t *testing.T) {
	db := testDB(t)
	txFunc := NewTxFunc(db)

	// Use a repo-style helper to get the tx from context.
	r := &repo{db: db}

	// Insert a user inside a transaction.
	err := txFunc(context.Background(), func(ctx context.Context) error {
		conn := r.conn(ctx)
		_, err := conn.ExecContext(ctx,
			"INSERT INTO users (id, username, password, role, created_at) VALUES (?, ?, ?, ?, ?)",
			"u1", "alice", "hashed", "user", 1000)
		return err
	})
	if err != nil {
		t.Fatalf("txFunc() error: %v", err)
	}

	// Verify insertion.
	var username string
	err = db.QueryRow("SELECT username FROM users WHERE id = ?", "u1").Scan(&username)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if username != "alice" {
		t.Errorf("username = %q, want alice", username)
	}
}

func TestNewTxFunc_Rollback(t *testing.T) {
	db := testDB(t)
	txFunc := NewTxFunc(db)
	r := &repo{db: db}

	// Fail inside transaction.
	err := txFunc(context.Background(), func(ctx context.Context) error {
		conn := r.conn(ctx)
		_, err := conn.ExecContext(ctx,
			"INSERT INTO users (id, username, password, role, created_at) VALUES (?, ?, ?, ?, ?)",
			"u2", "bob", "hashed", "user", 1000)
		if err != nil {
			return err
		}
		return fmt.Errorf("forced rollback")
	})
	if err == nil {
		t.Fatal("txFunc() should error")
	}

	// Verify rollback: user should not exist.
	var count int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", "u2").Scan(&count)
	if count != 0 {
		t.Errorf("user should not exist after rollback, got count=%d", count)
	}
}

func TestIsUniqueViolation(t *testing.T) {
	db := testDB(t)

	_, err := db.Exec(
		"INSERT INTO users (id, username, password, role, created_at) VALUES (?, ?, ?, ?, ?)",
		"u1", "alice", "hashed", "user", 1000)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Duplicate username.
	_, err = db.Exec(
		"INSERT INTO users (id, username, password, role, created_at) VALUES (?, ?, ?, ?, ?)",
		"u2", "alice", "hashed", "user", 1000)
	if err == nil {
		t.Fatal("expected unique violation error")
	}
	if !isUniqueViolation(err) {
		t.Errorf("isUniqueViolation() = false for: %v", err)
	}
}

func TestIsUniqueViolation_NonUnique(t *testing.T) {
	if isUniqueViolation(fmt.Errorf("some other error")) {
		t.Error("isUniqueViolation() should be false for non-SQLite errors")
	}
}
