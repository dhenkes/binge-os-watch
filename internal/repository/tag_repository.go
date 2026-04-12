package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/google/uuid"
)

type TagRepository struct {
	repo
}

var _ model.TagRepository = (*TagRepository)(nil)

func NewTagRepository(db DBTX) *TagRepository {
	return &TagRepository{repo{db: db}}
}

func (r *TagRepository) Create(ctx context.Context, tag *model.Tag) error {
	tag.ID = uuid.NewString()
	tag.CreatedAt = time.Now().UTC()

	_, err := r.conn(ctx).ExecContext(ctx,
		`INSERT INTO tag (id, user_id, name, created_at) VALUES (?, ?, ?, ?)`,
		tag.ID, tag.UserID, tag.Name, toUnix(tag.CreatedAt),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.NewAlreadyExists("tag already exists")
		}
		return fmt.Errorf("creating tag: %w", err)
	}
	return nil
}

func (r *TagRepository) GetByID(ctx context.Context, id string) (*model.Tag, error) {
	var t model.Tag
	var createdAt int64
	err := r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, user_id, name, created_at FROM tag WHERE id = ?`, id,
	).Scan(&t.ID, &t.UserID, &t.Name, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.NewNotFound("tag not found")
		}
		return nil, fmt.Errorf("getting tag: %w", err)
	}
	t.CreatedAt = fromUnix(createdAt)
	return &t, nil
}

func (r *TagRepository) ListByUser(ctx context.Context, userID string) ([]model.Tag, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT id, user_id, name, created_at FROM tag WHERE user_id = ? ORDER BY name`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}
	defer rows.Close()

	var tags []model.Tag
	for rows.Next() {
		var t model.Tag
		var createdAt int64
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning tag: %w", err)
		}
		t.CreatedAt = fromUnix(createdAt)
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *TagRepository) Delete(ctx context.Context, id string) error {
	_, err := r.conn(ctx).ExecContext(ctx, `DELETE FROM tag WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting tag: %w", err)
	}
	return nil
}
