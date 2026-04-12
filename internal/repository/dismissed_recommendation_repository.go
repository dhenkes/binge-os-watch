package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DismissedRecommendationRepository tracks dismissed recommendations per user.
type DismissedRecommendationRepository struct {
	repo
}

func NewDismissedRecommendationRepository(db DBTX) *DismissedRecommendationRepository {
	return &DismissedRecommendationRepository{repo{db: db}}
}

func (r *DismissedRecommendationRepository) Dismiss(ctx context.Context, userID string, tmdbID int, mediaType string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`INSERT INTO dismissed_recommendations (id, user_id, tmdb_id, media_type, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, tmdb_id, media_type) DO NOTHING`,
		uuid.NewString(), userID, tmdbID, mediaType, toUnix(time.Now().UTC()),
	)
	if err != nil {
		return fmt.Errorf("dismissing recommendation: %w", err)
	}
	return nil
}

func (r *DismissedRecommendationRepository) ListAll(ctx context.Context, userID string) (map[string]bool, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT tmdb_id, media_type FROM dismissed_recommendations WHERE user_id = ?`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing dismissed: %w", err)
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var tmdbID int
		var mediaType string
		if err := rows.Scan(&tmdbID, &mediaType); err != nil {
			continue
		}
		result[fmt.Sprintf("%d:%s", tmdbID, mediaType)] = true
	}
	return result, rows.Err()
}
