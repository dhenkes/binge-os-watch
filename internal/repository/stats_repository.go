package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// StatsRepository implements model.StatsRepository against the Option B
// schema (user_library / watch_event / rating_show / rating_movie).
type StatsRepository struct {
	repo
}

var _ model.StatsRepository = (*StatsRepository)(nil)

func NewStatsRepository(db DBTX) *StatsRepository {
	return &StatsRepository{repo{db: db}}
}

func (r *StatsRepository) GetUserStats(ctx context.Context, userID string) (*model.UserStats, error) {
	return r.getStats(ctx, userID)
}

func (r *StatsRepository) GetGlobalStats(ctx context.Context) (*model.UserStats, error) {
	return r.getStats(ctx, "")
}

// getStats computes the four UserStats fields. userID == "" computes a
// global aggregate across every user.
func (r *StatsRepository) getStats(ctx context.Context, userID string) (*model.UserStats, error) {
	conn := r.conn(ctx)
	s := &model.UserStats{}

	// TotalMovies: library entries explicitly marked watched. Movies never
	// auto-derive, so manual_status='watched' is the source of truth.
	movieCountQ := `SELECT COUNT(*) FROM user_library
		WHERE media_type='movie' AND manual_status='watched'`
	if userID != "" {
		movieCountQ += " AND user_id = ?"
		conn.QueryRowContext(ctx, movieCountQ, userID).Scan(&s.TotalMovies)
	} else {
		conn.QueryRowContext(ctx, movieCountQ).Scan(&s.TotalMovies)
	}

	// TotalEpisodes: distinct episode watch events (not counting rewatches
	// as separate episodes).
	epCountQ := `SELECT COUNT(DISTINCT episode_id) FROM watch_event WHERE episode_id IS NOT NULL`
	if userID != "" {
		epCountQ += " AND user_id = ?"
		conn.QueryRowContext(ctx, epCountQ, userID).Scan(&s.TotalEpisodes)
	} else {
		conn.QueryRowContext(ctx, epCountQ).Scan(&s.TotalEpisodes)
	}

	// Movie runtime: sum of catalog runtime for watched movies.
	var movieTime sql.NullInt64
	movieRtQ := `SELECT SUM(tm.runtime_minutes)
		FROM user_library ul
		JOIN tmdb_movie tm ON tm.id = ul.movie_id
		WHERE ul.media_type='movie' AND ul.manual_status='watched'`
	if userID != "" {
		movieRtQ += " AND ul.user_id = ?"
		conn.QueryRowContext(ctx, movieRtQ, userID).Scan(&movieTime)
	} else {
		conn.QueryRowContext(ctx, movieRtQ).Scan(&movieTime)
	}

	// Episode runtime: sum across distinct watched episodes (avoid double
	// counting rewatches).
	var episodeTime sql.NullInt64
	epRtQ := `SELECT SUM(te.runtime_minutes) FROM tmdb_episode te
		WHERE te.id IN (
			SELECT DISTINCT episode_id FROM watch_event
			WHERE episode_id IS NOT NULL`
	if userID != "" {
		epRtQ += " AND user_id = ?"
		epRtQ += ")"
		conn.QueryRowContext(ctx, epRtQ, userID).Scan(&episodeTime)
	} else {
		epRtQ += ")"
		conn.QueryRowContext(ctx, epRtQ).Scan(&episodeTime)
	}

	s.TotalTimeMinutes = int(movieTime.Int64) + int(episodeTime.Int64)

	// AvgRating: pooled average across show + movie subject ratings.
	var avgRating sql.NullFloat64
	avgQ := `SELECT AVG(CAST(score AS REAL)) FROM (
		SELECT score FROM rating_show`
	if userID != "" {
		avgQ += " WHERE user_id = ?"
	}
	avgQ += ` UNION ALL SELECT score FROM rating_movie`
	if userID != "" {
		avgQ += " WHERE user_id = ?"
		avgQ += ")"
		conn.QueryRowContext(ctx, avgQ, userID, userID).Scan(&avgRating)
	} else {
		avgQ += ")"
		conn.QueryRowContext(ctx, avgQ).Scan(&avgRating)
	}
	s.AvgRating = avgRating.Float64

	return s, nil
}

func (r *StatsRepository) GetAdminStats(ctx context.Context) (*model.AdminStats, error) {
	conn := r.conn(ctx)
	s := &model.AdminStats{}

	conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&s.TotalUsers)

	now := time.Now().UTC()
	dayAgo := now.Add(-24 * time.Hour).Unix()
	weekAgo := now.Add(-7 * 24 * time.Hour).Unix()
	monthAgo := now.Add(-30 * 24 * time.Hour).Unix()

	conn.QueryRowContext(ctx, `SELECT COUNT(DISTINCT user_id) FROM sessions WHERE last_seen_at >= ?`, dayAgo).Scan(&s.ActiveUsersDay)
	conn.QueryRowContext(ctx, `SELECT COUNT(DISTINCT user_id) FROM sessions WHERE last_seen_at >= ?`, weekAgo).Scan(&s.ActiveUsersWeek)
	conn.QueryRowContext(ctx, `SELECT COUNT(DISTINCT user_id) FROM sessions WHERE last_seen_at >= ?`, monthAgo).Scan(&s.ActiveUsersMonth)

	conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_library WHERE created_at >= ?`, dayAgo).Scan(&s.NewMediaDay)
	conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_library WHERE created_at >= ?`, weekAgo).Scan(&s.NewMediaWeek)
	conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_library WHERE created_at >= ?`, monthAgo).Scan(&s.NewMediaMonth)

	return s, nil
}

// GetLastActiveByUser returns the most recent session activity per user.
func (r *StatsRepository) GetLastActiveByUser(ctx context.Context) (map[string]time.Time, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT user_id, MAX(last_seen_at) FROM sessions GROUP BY user_id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]time.Time)
	for rows.Next() {
		var userID string
		var lastSeen int64
		if err := rows.Scan(&userID, &lastSeen); err != nil {
			continue
		}
		result[userID] = fromUnix(lastSeen)
	}
	return result, rows.Err()
}
