package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/google/uuid"
)

// RatingV2Repository is the per-subject rating store that replaces the
// polymorphic ratings table. Each subject gets its own backing table with
// a proper foreign key so deletes cascade correctly.
type RatingV2Repository struct {
	repo
}

var _ model.RatingRepositoryV2 = (*RatingV2Repository)(nil)

func NewRatingV2Repository(db DBTX) *RatingV2Repository {
	return &RatingV2Repository{repo{db: db}}
}

// upsert is the shared upsert logic — table name and subject column vary.
func (r *RatingV2Repository) upsert(ctx context.Context, table, subjectCol, userID, subjectID string, score int) error {
	query := fmt.Sprintf(
		`INSERT INTO %s (id, user_id, %s, score, created_at, updated_at)
		 VALUES (?, ?, ?, ?, strftime('%%s','now'), strftime('%%s','now'))
		 ON CONFLICT(user_id, %s) DO UPDATE SET
		     score = excluded.score,
		     updated_at = strftime('%%s','now')`,
		table, subjectCol, subjectCol)
	_, err := r.conn(ctx).ExecContext(ctx, query, uuid.NewString(), userID, subjectID, score)
	if err != nil {
		return fmt.Errorf("upserting %s rating: %w", table, err)
	}
	return nil
}

func (r *RatingV2Repository) deleteOne(ctx context.Context, table, subjectCol, userID, subjectID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE user_id = ? AND %s = ?`, table, subjectCol),
		userID, subjectID)
	if err != nil {
		return fmt.Errorf("deleting %s rating: %w", table, err)
	}
	return nil
}

// --- Show ---

func (r *RatingV2Repository) UpsertShow(ctx context.Context, userID, showID string, score int) error {
	return r.upsert(ctx, "rating_show", "show_id", userID, showID, score)
}

func (r *RatingV2Repository) GetShow(ctx context.Context, userID, showID string) (*model.RatingShow, error) {
	var rs model.RatingShow
	err := r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, user_id, show_id, score, created_at, updated_at
		 FROM rating_show WHERE user_id = ? AND show_id = ?`,
		userID, showID).Scan(&rs.ID, &rs.UserID, &rs.ShowID, &rs.Score, &rs.CreatedAt, &rs.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &rs, nil
}

func (r *RatingV2Repository) DeleteShow(ctx context.Context, userID, showID string) error {
	return r.deleteOne(ctx, "rating_show", "show_id", userID, showID)
}

// --- Movie ---

func (r *RatingV2Repository) UpsertMovie(ctx context.Context, userID, movieID string, score int) error {
	return r.upsert(ctx, "rating_movie", "movie_id", userID, movieID, score)
}

func (r *RatingV2Repository) GetMovie(ctx context.Context, userID, movieID string) (*model.RatingMovie, error) {
	var rm model.RatingMovie
	err := r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, user_id, movie_id, score, created_at, updated_at
		 FROM rating_movie WHERE user_id = ? AND movie_id = ?`,
		userID, movieID).Scan(&rm.ID, &rm.UserID, &rm.MovieID, &rm.Score, &rm.CreatedAt, &rm.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &rm, nil
}

func (r *RatingV2Repository) DeleteMovie(ctx context.Context, userID, movieID string) error {
	return r.deleteOne(ctx, "rating_movie", "movie_id", userID, movieID)
}

// --- Season ---

func (r *RatingV2Repository) UpsertSeason(ctx context.Context, userID, seasonID string, score int) error {
	return r.upsert(ctx, "rating_season", "season_id", userID, seasonID, score)
}

func (r *RatingV2Repository) GetSeason(ctx context.Context, userID, seasonID string) (*model.RatingSeasonV2, error) {
	var rs model.RatingSeasonV2
	err := r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, user_id, season_id, score, created_at, updated_at
		 FROM rating_season WHERE user_id = ? AND season_id = ?`,
		userID, seasonID).Scan(&rs.ID, &rs.UserID, &rs.SeasonID, &rs.Score, &rs.CreatedAt, &rs.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &rs, nil
}

func (r *RatingV2Repository) DeleteSeason(ctx context.Context, userID, seasonID string) error {
	return r.deleteOne(ctx, "rating_season", "season_id", userID, seasonID)
}

// --- Episode ---

func (r *RatingV2Repository) UpsertEpisode(ctx context.Context, userID, episodeID string, score int) error {
	return r.upsert(ctx, "rating_episode", "episode_id", userID, episodeID, score)
}

func (r *RatingV2Repository) GetEpisode(ctx context.Context, userID, episodeID string) (*model.RatingEpisodeV2, error) {
	var re model.RatingEpisodeV2
	err := r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, user_id, episode_id, score, created_at, updated_at
		 FROM rating_episode WHERE user_id = ? AND episode_id = ?`,
		userID, episodeID).Scan(&re.ID, &re.UserID, &re.EpisodeID, &re.Score, &re.CreatedAt, &re.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &re, nil
}

func (r *RatingV2Repository) DeleteEpisode(ctx context.Context, userID, episodeID string) error {
	return r.deleteOne(ctx, "rating_episode", "episode_id", userID, episodeID)
}

// --- Batch helpers ---

func (r *RatingV2Repository) ListShowRatingsByIDs(ctx context.Context, userID string, showIDs []string) (map[string]int, error) {
	return r.batchByIDs(ctx, "rating_show", "show_id", userID, showIDs)
}

func (r *RatingV2Repository) ListMovieRatingsByIDs(ctx context.Context, userID string, movieIDs []string) (map[string]int, error) {
	return r.batchByIDs(ctx, "rating_movie", "movie_id", userID, movieIDs)
}

func (r *RatingV2Repository) ListSeasonRatingsByShow(ctx context.Context, userID, showID string) (map[string]int, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT rs.season_id, rs.score
		 FROM rating_season rs
		 JOIN tmdb_season sn ON sn.id = rs.season_id
		 WHERE rs.user_id = ? AND sn.show_id = ?`, userID, showID)
	if err != nil {
		return nil, err
	}
	return scanIDScoreMap(rows)
}

func (r *RatingV2Repository) ListEpisodeRatingsByShow(ctx context.Context, userID, showID string) (map[string]int, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT re.episode_id, re.score
		 FROM rating_episode re
		 JOIN tmdb_episode e ON e.id = re.episode_id
		 JOIN tmdb_season sn ON sn.id = e.season_id
		 WHERE re.user_id = ? AND sn.show_id = ?`, userID, showID)
	if err != nil {
		return nil, err
	}
	return scanIDScoreMap(rows)
}

func (r *RatingV2Repository) batchByIDs(ctx context.Context, table, col, userID string, ids []string) (map[string]int, error) {
	if len(ids) == 0 {
		return map[string]int{}, nil
	}
	placeholders := make([]string, len(ids))
	args := []any{userID}
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := fmt.Sprintf(`SELECT %s, score FROM %s WHERE user_id = ? AND %s IN (%s)`,
		col, table, col, strings.Join(placeholders, ","))
	rows, err := r.conn(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return scanIDScoreMap(rows)
}

func scanIDScoreMap(rows *sql.Rows) (map[string]int, error) {
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var score int
		if err := rows.Scan(&id, &score); err != nil {
			return nil, err
		}
		out[id] = score
	}
	return out, rows.Err()
}
