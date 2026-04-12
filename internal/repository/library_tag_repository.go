package repository

import (
	"context"
	"fmt"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

type LibraryTagRepository struct {
	repo
}

var _ model.LibraryTagRepository = (*LibraryTagRepository)(nil)

func NewLibraryTagRepository(db DBTX) *LibraryTagRepository {
	return &LibraryTagRepository{repo{db: db}}
}

func (r *LibraryTagRepository) Add(ctx context.Context, libraryID, tagID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`INSERT INTO library_tag (library_id, tag_id) VALUES (?, ?)
		 ON CONFLICT(library_id, tag_id) DO NOTHING`,
		libraryID, tagID)
	if err != nil {
		return fmt.Errorf("adding library tag: %w", err)
	}
	return nil
}

func (r *LibraryTagRepository) Remove(ctx context.Context, libraryID, tagID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`DELETE FROM library_tag WHERE library_id = ? AND tag_id = ?`,
		libraryID, tagID)
	if err != nil {
		return fmt.Errorf("removing library tag: %w", err)
	}
	return nil
}

func (r *LibraryTagRepository) ListByLibrary(ctx context.Context, libraryID string) ([]model.Tag, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT t.id, t.user_id, t.name, t.created_at
		 FROM tag t JOIN library_tag lt ON t.id = lt.tag_id
		 WHERE lt.library_id = ?
		 ORDER BY t.name`, libraryID)
	if err != nil {
		return nil, fmt.Errorf("listing tags by library: %w", err)
	}
	defer rows.Close()
	var out []model.Tag
	for rows.Next() {
		var t model.Tag
		var createdAt int64
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &createdAt); err != nil {
			return nil, err
		}
		t.CreatedAt = fromUnix(createdAt)
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListByUser returns a map of libraryID → tags for the whole user's
// library, in one query. Used by the library grid so every card can show
// its tags without N+1.
func (r *LibraryTagRepository) ListByUser(ctx context.Context, userID string) (map[string][]model.Tag, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT lt.library_id, t.id, t.user_id, t.name, t.created_at
		 FROM library_tag lt
		 JOIN tag t ON t.id = lt.tag_id
		 JOIN user_library ul ON ul.id = lt.library_id
		 WHERE ul.user_id = ?
		 ORDER BY lt.library_id, t.name`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing tags by user: %w", err)
	}
	defer rows.Close()
	out := map[string][]model.Tag{}
	for rows.Next() {
		var libID string
		var t model.Tag
		var createdAt int64
		if err := rows.Scan(&libID, &t.ID, &t.UserID, &t.Name, &createdAt); err != nil {
			return nil, err
		}
		t.CreatedAt = fromUnix(createdAt)
		out[libID] = append(out[libID], t)
	}
	return out, rows.Err()
}
