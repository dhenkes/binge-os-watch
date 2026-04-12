package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/google/uuid"
)

type TMDBJobRepository struct {
	repo
}

var _ model.TMDBJobRepository = (*TMDBJobRepository)(nil)

func NewTMDBJobRepository(db DBTX) *TMDBJobRepository {
	return &TMDBJobRepository{repo{db: db}}
}

func (r *TMDBJobRepository) Create(ctx context.Context, j *model.TMDBJob) error {
	if j.ID == "" {
		j.ID = uuid.NewString()
	}
	var userID any
	if j.UserID != nil {
		userID = *j.UserID
	}
	_, err := r.conn(ctx).ExecContext(ctx,
		`INSERT INTO tmdb_job (id, user_id, kind, payload, status, error, created_at, started_at)
		 VALUES (?, ?, ?, ?, 'pending', '', strftime('%s','now'), NULL)`,
		j.ID, userID, j.Kind, j.Payload)
	if err != nil {
		return fmt.Errorf("creating tmdb job: %w", err)
	}
	return nil
}

func (r *TMDBJobRepository) MarkRunning(ctx context.Context, id string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE tmdb_job SET status='running', started_at=strftime('%s','now'), error='' WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("marking tmdb job running: %w", err)
	}
	return nil
}

func (r *TMDBJobRepository) MarkFailed(ctx context.Context, id, errMsg string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE tmdb_job SET status='failed', error=? WHERE id = ?`, errMsg, id)
	if err != nil {
		return fmt.Errorf("marking tmdb job failed: %w", err)
	}
	return nil
}

func (r *TMDBJobRepository) Delete(ctx context.Context, id string) error {
	_, err := r.conn(ctx).ExecContext(ctx, `DELETE FROM tmdb_job WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting tmdb job: %w", err)
	}
	return nil
}

func (r *TMDBJobRepository) ListAll(ctx context.Context) ([]model.TMDBJob, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT id, user_id, kind, payload, status, error, created_at, started_at
		 FROM tmdb_job ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing tmdb jobs: %w", err)
	}
	defer rows.Close()
	var out []model.TMDBJob
	for rows.Next() {
		var j model.TMDBJob
		var userID sql.NullString
		var startedAt sql.NullInt64
		if err := rows.Scan(&j.ID, &userID, &j.Kind, &j.Payload, &j.Status, &j.Error, &j.CreatedAt, &startedAt); err != nil {
			return nil, err
		}
		if userID.Valid {
			v := userID.String
			j.UserID = &v
		}
		if startedAt.Valid {
			v := startedAt.Int64
			j.StartedAt = &v
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
