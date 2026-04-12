package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/google/uuid"
)

type LibraryImportJobRepository struct {
	repo
}

var _ model.LibraryImportJobRepository = (*LibraryImportJobRepository)(nil)

func NewLibraryImportJobRepository(db DBTX) *LibraryImportJobRepository {
	return &LibraryImportJobRepository{repo{db: db}}
}

func (r *LibraryImportJobRepository) Create(ctx context.Context, j *model.LibraryImportJob) error {
	if j.ID == "" {
		j.ID = uuid.NewString()
	}
	_, err := r.conn(ctx).ExecContext(ctx,
		`INSERT INTO library_import_job (id, user_id, payload, status, error, created_at, started_at)
		 VALUES (?, ?, ?, 'pending', '', strftime('%s','now'), NULL)`,
		j.ID, j.UserID, j.Payload)
	if err != nil {
		return fmt.Errorf("creating import job: %w", err)
	}
	return nil
}

func (r *LibraryImportJobRepository) MarkRunning(ctx context.Context, id string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE library_import_job SET status='running', started_at=strftime('%s','now'), error='' WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("marking job running: %w", err)
	}
	return nil
}

func (r *LibraryImportJobRepository) MarkFailed(ctx context.Context, id, errMsg string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE library_import_job SET status='failed', error=? WHERE id = ?`, errMsg, id)
	if err != nil {
		return fmt.Errorf("marking job failed: %w", err)
	}
	return nil
}

func (r *LibraryImportJobRepository) Delete(ctx context.Context, id string) error {
	_, err := r.conn(ctx).ExecContext(ctx, `DELETE FROM library_import_job WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting import job: %w", err)
	}
	return nil
}

func (r *LibraryImportJobRepository) ListAll(ctx context.Context) ([]model.LibraryImportJob, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT id, user_id, payload, status, error, created_at, started_at
		 FROM library_import_job ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing import jobs: %w", err)
	}
	defer rows.Close()
	var out []model.LibraryImportJob
	for rows.Next() {
		var j model.LibraryImportJob
		var startedAt sql.NullInt64
		if err := rows.Scan(&j.ID, &j.UserID, &j.Payload, &j.Status, &j.Error, &j.CreatedAt, &startedAt); err != nil {
			return nil, err
		}
		if startedAt.Valid {
			v := startedAt.Int64
			j.StartedAt = &v
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
