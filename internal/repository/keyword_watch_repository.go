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

type KeywordWatchRepository struct {
	repo
}

var _ model.KeywordWatchRepository = (*KeywordWatchRepository)(nil)

func NewKeywordWatchRepository(db DBTX) *KeywordWatchRepository {
	return &KeywordWatchRepository{repo{db: db}}
}

func (r *KeywordWatchRepository) Create(ctx context.Context, kw *model.KeywordWatch) error {
	kw.ID = uuid.NewString()
	kw.CreatedAt = time.Now().UTC()

	_, err := r.conn(ctx).ExecContext(ctx,
		`INSERT INTO keyword_watches (id, user_id, keyword, media_types, created_at) VALUES (?, ?, ?, ?, ?)`,
		kw.ID, kw.UserID, kw.Keyword, kw.MediaTypes, toUnix(kw.CreatedAt),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.NewAlreadyExists("keyword watch already exists")
		}
		return fmt.Errorf("creating keyword watch: %w", err)
	}
	return nil
}

func (r *KeywordWatchRepository) GetByID(ctx context.Context, id string) (*model.KeywordWatch, error) {
	var kw model.KeywordWatch
	var createdAt int64
	err := r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, user_id, keyword, media_types, created_at FROM keyword_watches WHERE id = ?`, id,
	).Scan(&kw.ID, &kw.UserID, &kw.Keyword, &kw.MediaTypes, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.NewNotFound("keyword watch not found")
		}
		return nil, fmt.Errorf("getting keyword watch: %w", err)
	}
	kw.CreatedAt = fromUnix(createdAt)
	return &kw, nil
}

// ListAll returns all keyword watches across all users. Used by the keyword scanner job.
func (r *KeywordWatchRepository) ListAll(ctx context.Context) ([]model.KeywordWatch, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT id, user_id, keyword, media_types, created_at FROM keyword_watches ORDER BY user_id, keyword`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing all keyword watches: %w", err)
	}
	defer rows.Close()
	return r.scanWatches(rows)
}

func (r *KeywordWatchRepository) ListByUser(ctx context.Context, userID string) ([]model.KeywordWatch, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT id, user_id, keyword, media_types, created_at FROM keyword_watches WHERE user_id = ? ORDER BY keyword`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing keyword watches: %w", err)
	}
	defer rows.Close()
	return r.scanWatches(rows)
}

func (r *KeywordWatchRepository) scanWatches(rows *sql.Rows) ([]model.KeywordWatch, error) {
	var watches []model.KeywordWatch
	for rows.Next() {
		var kw model.KeywordWatch
		var createdAt int64
		if err := rows.Scan(&kw.ID, &kw.UserID, &kw.Keyword, &kw.MediaTypes, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning keyword watch: %w", err)
		}
		kw.CreatedAt = fromUnix(createdAt)
		watches = append(watches, kw)
	}
	return watches, rows.Err()
}

func (r *KeywordWatchRepository) Update(ctx context.Context, kw *model.KeywordWatch, mask []string) error {
	allowed := map[string]any{
		"keyword":     kw.Keyword,
		"media_types": kw.MediaTypes,
	}
	sets, args := buildUpdateClauses(mask, allowed)
	if len(sets) == 0 {
		return nil
	}
	args = append(args, kw.ID)

	_, err := r.conn(ctx).ExecContext(ctx,
		fmt.Sprintf("UPDATE keyword_watches SET %s WHERE id = ?", joinStrings(sets, ", ")), args...,
	)
	if err != nil {
		return fmt.Errorf("updating keyword watch: %w", err)
	}
	return nil
}

func (r *KeywordWatchRepository) Delete(ctx context.Context, id string) error {
	_, err := r.conn(ctx).ExecContext(ctx, `DELETE FROM keyword_watches WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting keyword watch: %w", err)
	}
	return nil
}

func (r *KeywordWatchRepository) CreateResult(ctx context.Context, result *model.KeywordResult) error {
	result.ID = uuid.NewString()
	result.CreatedAt = time.Now().UTC()

	_, err := r.conn(ctx).ExecContext(ctx,
		`INSERT INTO keyword_results (id, keyword_watch_id, tmdb_id, media_type, title, poster_path, release_date, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(keyword_watch_id, tmdb_id, media_type) DO NOTHING`,
		result.ID, result.KeywordWatchID, result.TMDBID, string(result.MediaType),
		result.Title, result.PosterPath, result.ReleaseDate,
		string(result.Status), toUnix(result.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("creating keyword result: %w", err)
	}
	return nil
}

func (r *KeywordWatchRepository) ListPendingResults(ctx context.Context, userID string) ([]model.KeywordResult, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT kr.id, kr.keyword_watch_id, kr.tmdb_id, kr.media_type, kr.title, kr.poster_path, kr.release_date, kr.status, kr.created_at
		 FROM keyword_results kr
		 JOIN keyword_watches kw ON kr.keyword_watch_id = kw.id
		 WHERE kw.user_id = ? AND kr.status = 'pending'
		 ORDER BY kr.created_at DESC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing pending results: %w", err)
	}
	defer rows.Close()

	var results []model.KeywordResult
	for rows.Next() {
		var kr model.KeywordResult
		var mt, status string
		var createdAt int64
		if err := rows.Scan(&kr.ID, &kr.KeywordWatchID, &kr.TMDBID, &mt, &kr.Title, &kr.PosterPath, &kr.ReleaseDate, &status, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning keyword result: %w", err)
		}
		kr.MediaType = model.MediaType(mt)
		kr.Status = model.KeywordResultStatus(status)
		kr.CreatedAt = fromUnix(createdAt)
		results = append(results, kr)
	}
	return results, rows.Err()
}

func (r *KeywordWatchRepository) ListDismissedResults(ctx context.Context, userID string) ([]model.KeywordResult, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT kr.id, kr.keyword_watch_id, kr.tmdb_id, kr.media_type, kr.title, kr.poster_path, kr.release_date, kr.status, kr.created_at
		 FROM keyword_results kr
		 JOIN keyword_watches kw ON kr.keyword_watch_id = kw.id
		 WHERE kw.user_id = ? AND kr.status = 'dismissed'
		 ORDER BY kr.created_at DESC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing dismissed results: %w", err)
	}
	defer rows.Close()

	var results []model.KeywordResult
	for rows.Next() {
		var kr model.KeywordResult
		var mt, status string
		var createdAt int64
		if err := rows.Scan(&kr.ID, &kr.KeywordWatchID, &kr.TMDBID, &mt, &kr.Title, &kr.PosterPath, &kr.ReleaseDate, &status, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning keyword result: %w", err)
		}
		kr.MediaType = model.MediaType(mt)
		kr.Status = model.KeywordResultStatus(status)
		kr.CreatedAt = fromUnix(createdAt)
		results = append(results, kr)
	}
	return results, rows.Err()
}

func (r *KeywordWatchRepository) UpdateResultStatus(ctx context.Context, id string, status model.KeywordResultStatus) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE keyword_results SET status = ? WHERE id = ?`, string(status), id,
	)
	if err != nil {
		return fmt.Errorf("updating result status: %w", err)
	}
	return nil
}

func (r *KeywordWatchRepository) DismissAllResults(ctx context.Context, keywordWatchID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE keyword_results SET status = 'dismissed' WHERE keyword_watch_id = ? AND status = 'pending'`,
		keywordWatchID,
	)
	if err != nil {
		return fmt.Errorf("dismissing all results: %w", err)
	}
	return nil
}

func (r *KeywordWatchRepository) PendingCount(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.conn(ctx).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM keyword_results kr
		 JOIN keyword_watches kw ON kr.keyword_watch_id = kw.id
		 WHERE kw.user_id = ? AND kr.status = 'pending'`, userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting pending results: %w", err)
	}
	return count, nil
}

func (r *KeywordWatchRepository) ResultExists(ctx context.Context, keywordWatchID string, tmdbID int, mediaType model.MediaType) (bool, error) {
	var count int
	err := r.conn(ctx).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM keyword_results WHERE keyword_watch_id = ? AND tmdb_id = ? AND media_type = ?`,
		keywordWatchID, tmdbID, string(mediaType),
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking result exists: %w", err)
	}
	return count > 0, nil
}

func joinStrings(s []string, sep string) string {
	result := ""
	for i, v := range s {
		if i > 0 {
			result += sep
		}
		result += v
	}
	return result
}
