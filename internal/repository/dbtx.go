package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/dhenkes/binge-os-watch/internal/model"
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type txKey struct{}

// DBTX is the common interface satisfied by both *sql.DB and *sql.Tx.
// All repositories accept this so they can operate inside or outside a transaction.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// repo is a base type embedded by all repositories. Its conn method
// returns the *sql.Tx from the context if present, otherwise the
// repository's own DBTX. This lets repositories transparently participate
// in a transaction started by the service layer via TxFunc.
type repo struct{ db DBTX }

func (r *repo) conn(ctx context.Context) DBTX {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return r.db
}

// NewTxFunc returns a model.TxFunc backed by the given *sql.DB.
// It stores the *sql.Tx in the context so repositories pick it up via conn().
func NewTxFunc(db *sql.DB) model.TxFunc {
	return func(ctx context.Context, fn func(ctx context.Context) error) error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("beginning transaction: %w", err)
		}
		defer tx.Rollback()

		txCtx := context.WithValue(ctx, txKey{}, tx)
		if err := fn(txCtx); err != nil {
			return err
		}
		return tx.Commit()
	}
}

// beginTx starts a new transaction if db is a *sql.DB, or returns the
// existing *sql.Tx if already inside one. The returned bool indicates
// whether the caller owns the transaction (and should commit/rollback).
func beginTx(ctx context.Context, db DBTX) (*sql.Tx, bool, error) {
	if tx, ok := db.(*sql.Tx); ok {
		return tx, false, nil
	}
	sqlDB, ok := db.(*sql.DB)
	if !ok {
		return nil, false, fmt.Errorf("cannot begin transaction: unsupported DBTX type %T", db)
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("beginning transaction: %w", err)
	}
	return tx, true, nil
}

// finishTx commits or rolls back a transaction that the caller owns.
func finishTx(tx *sql.Tx, owns bool, err error) error {
	if !owns {
		return err
	}
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// isUniqueViolation returns true if err is a SQLite UNIQUE constraint violation.
func isUniqueViolation(err error) bool {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
	}
	return false
}
