package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/google/uuid"
)

type WatchEventRepository struct {
	repo
}

var _ model.WatchEventRepository = (*WatchEventRepository)(nil)

func NewWatchEventRepository(db DBTX) *WatchEventRepository {
	return &WatchEventRepository{repo{db: db}}
}

func (r *WatchEventRepository) Create(ctx context.Context, e *model.WatchEvent) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	// ON CONFLICT against the partial unique indexes makes a re-import
	// with the same (user, subject, watched_at) a silent no-op. Real
	// rewatches use a different timestamp and insert cleanly.
	_, err := r.conn(ctx).ExecContext(ctx,
		`INSERT INTO watch_event (id, user_id, episode_id, movie_id, watched_at, notes)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT DO NOTHING`,
		e.ID, e.UserID, e.EpisodeID, e.MovieID, e.WatchedAt, e.Notes,
	)
	if err != nil {
		return fmt.Errorf("creating watch event: %w", err)
	}
	return nil
}

func (r *WatchEventRepository) Delete(ctx context.Context, id string) error {
	_, err := r.conn(ctx).ExecContext(ctx, `DELETE FROM watch_event WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting watch event: %w", err)
	}
	return nil
}

// DeleteLatestForEpisode removes only the most recent watch of an episode
// — used by the "unwatch" affordance so the user can walk a rewatch back
// one click at a time.
func (r *WatchEventRepository) DeleteLatestForEpisode(ctx context.Context, userID, episodeID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`DELETE FROM watch_event
		 WHERE id = (
		   SELECT id FROM watch_event
		   WHERE user_id = ? AND episode_id = ?
		   ORDER BY watched_at DESC, id DESC LIMIT 1
		 )`, userID, episodeID)
	if err != nil {
		return fmt.Errorf("deleting latest episode watch: %w", err)
	}
	return nil
}

func (r *WatchEventRepository) DeleteAllForEpisode(ctx context.Context, userID, episodeID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`DELETE FROM watch_event WHERE user_id = ? AND episode_id = ?`,
		userID, episodeID)
	if err != nil {
		return fmt.Errorf("deleting all episode watches: %w", err)
	}
	return nil
}

func (r *WatchEventRepository) DeleteLatestForMovie(ctx context.Context, userID, movieID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`DELETE FROM watch_event
		 WHERE id = (
		   SELECT id FROM watch_event
		   WHERE user_id = ? AND movie_id = ?
		   ORDER BY watched_at DESC, id DESC LIMIT 1
		 )`, userID, movieID)
	if err != nil {
		return fmt.Errorf("deleting latest movie watch: %w", err)
	}
	return nil
}

func (r *WatchEventRepository) DeleteAllForMovie(ctx context.Context, userID, movieID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`DELETE FROM watch_event WHERE user_id = ? AND movie_id = ?`,
		userID, movieID)
	if err != nil {
		return fmt.Errorf("deleting all movie watches: %w", err)
	}
	return nil
}

func (r *WatchEventRepository) LatestForEpisode(ctx context.Context, userID, episodeID string) (*model.WatchEvent, error) {
	return scanWatchEventOne(r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, user_id, episode_id, movie_id, watched_at, notes
		 FROM watch_event WHERE user_id = ? AND episode_id = ?
		 ORDER BY watched_at DESC, id DESC LIMIT 1`,
		userID, episodeID))
}

func (r *WatchEventRepository) LatestForMovie(ctx context.Context, userID, movieID string) (*model.WatchEvent, error) {
	return scanWatchEventOne(r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, user_id, episode_id, movie_id, watched_at, notes
		 FROM watch_event WHERE user_id = ? AND movie_id = ?
		 ORDER BY watched_at DESC, id DESC LIMIT 1`,
		userID, movieID))
}

func (r *WatchEventRepository) HasEpisode(ctx context.Context, userID, episodeID string) (bool, error) {
	var n int
	err := r.conn(ctx).QueryRowContext(ctx,
		`SELECT 1 FROM watch_event WHERE user_id = ? AND episode_id = ? LIMIT 1`,
		userID, episodeID).Scan(&n)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *WatchEventRepository) HasMovie(ctx context.Context, userID, movieID string) (bool, error) {
	var n int
	err := r.conn(ctx).QueryRowContext(ctx,
		`SELECT 1 FROM watch_event WHERE user_id = ? AND movie_id = ? LIMIT 1`,
		userID, movieID).Scan(&n)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ProgressForShow returns the number of distinct watched aired-regular
// episodes, the total count of aired-regular episodes for a show, and
// whether the show has future (unaired, non-special) episodes. Distinct
// because rewatches don't count as additional progress.
func (r *WatchEventRepository) ProgressForShow(ctx context.Context, userID, showID string) (watched, total int, hasFuture bool, err error) {
	var futureCount int
	err = r.conn(ctx).QueryRowContext(ctx,
		`SELECT
		   (SELECT COUNT(*) FROM aired_regular_episodes e
		    JOIN tmdb_season sn ON e.season_id = sn.id
		    WHERE sn.show_id = ?) AS total,
		   (SELECT COUNT(DISTINCT e.id) FROM aired_regular_episodes e
		    JOIN tmdb_season sn ON e.season_id = sn.id
		    JOIN watch_event we ON we.episode_id = e.id
		    WHERE sn.show_id = ? AND we.user_id = ?) AS watched,
		   (SELECT COUNT(*) FROM tmdb_episode e
		    JOIN tmdb_season sn ON e.season_id = sn.id
		    WHERE sn.show_id = ? AND sn.season_number > 0
		      AND (e.air_date IS NULL OR e.air_date > CAST(strftime('%s','now') AS INTEGER))) AS future`,
		showID, showID, userID, showID).Scan(&total, &watched, &futureCount)
	if err != nil {
		return 0, 0, false, fmt.Errorf("computing show progress: %w", err)
	}
	return watched, total, futureCount > 0, nil
}

// NextUnwatched returns the earliest aired-regular episode in a show that
// the user has not yet watched. Specials and future episodes are excluded
// via the aired_regular_episodes view.
func (r *WatchEventRepository) NextUnwatched(ctx context.Context, userID, showID string) (*model.TMDBEpisode, error) {
	row := r.conn(ctx).QueryRowContext(ctx,
		`SELECT e.id, e.season_id, e.tmdb_episode_id, e.episode_number, e.name, e.overview, e.still_path, e.air_date, e.runtime_minutes
		 FROM aired_regular_episodes e
		 JOIN tmdb_season sn ON e.season_id = sn.id
		 WHERE sn.show_id = ?
		   AND NOT EXISTS (
		     SELECT 1 FROM watch_event we
		     WHERE we.episode_id = e.id AND we.user_id = ?
		   )
		 ORDER BY sn.season_number, e.episode_number
		 LIMIT 1`, showID, userID)
	return scanTMDBEpisodeOne(row)
}

// WatchedMapForShow returns a map of episode_id → most-recent watched_at
// for the given user+show. Used by the media detail page to render all
// episode rows without per-episode queries.
func (r *WatchEventRepository) WatchedMapForShow(ctx context.Context, userID, showID string) (map[string]int64, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT we.episode_id, MAX(we.watched_at)
		 FROM watch_event we
		 JOIN tmdb_episode e ON we.episode_id = e.id
		 JOIN tmdb_season sn ON e.season_id = sn.id
		 WHERE sn.show_id = ? AND we.user_id = ?
		 GROUP BY we.episode_id`, showID, userID)
	if err != nil {
		return nil, fmt.Errorf("watched map: %w", err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var id string
		var at int64
		if err := rows.Scan(&id, &at); err != nil {
			return nil, err
		}
		out[id] = at
	}
	return out, rows.Err()
}

func (r *WatchEventRepository) ListForEpisode(ctx context.Context, userID, episodeID string) ([]model.WatchEvent, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT id, user_id, episode_id, movie_id, watched_at, notes
		 FROM watch_event WHERE user_id = ? AND episode_id = ?
		 ORDER BY watched_at DESC`, userID, episodeID)
	if err != nil {
		return nil, fmt.Errorf("listing episode events: %w", err)
	}
	return scanWatchEvents(rows)
}

func (r *WatchEventRepository) ListForMovie(ctx context.Context, userID, movieID string) ([]model.WatchEvent, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT id, user_id, episode_id, movie_id, watched_at, notes
		 FROM watch_event WHERE user_id = ? AND movie_id = ?
		 ORDER BY watched_at DESC`, userID, movieID)
	if err != nil {
		return nil, fmt.Errorf("listing movie events: %w", err)
	}
	return scanWatchEvents(rows)
}

// UpdateLatestNotesForEpisode updates the notes field on the most recent
// watch_event row for (user, episode). Returns true if a row was updated.
func (r *WatchEventRepository) UpdateLatestNotesForEpisode(ctx context.Context, userID, episodeID, notes string) (bool, error) {
	res, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE watch_event SET notes = ?
		 WHERE id = (
		   SELECT id FROM watch_event
		   WHERE user_id = ? AND episode_id = ?
		   ORDER BY watched_at DESC, id DESC LIMIT 1
		 )`, notes, userID, episodeID)
	if err != nil {
		return false, fmt.Errorf("updating episode notes: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func scanWatchEventOne(row scannable) (*model.WatchEvent, error) {
	var e model.WatchEvent
	var epID, movID sql.NullString
	err := row.Scan(&e.ID, &e.UserID, &epID, &movID, &e.WatchedAt, &e.Notes)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.NewNotFound("watch event not found")
		}
		return nil, err
	}
	if epID.Valid {
		s := epID.String
		e.EpisodeID = &s
	}
	if movID.Valid {
		s := movID.String
		e.MovieID = &s
	}
	return &e, nil
}

func scanWatchEvents(rows *sql.Rows) ([]model.WatchEvent, error) {
	defer rows.Close()
	var out []model.WatchEvent
	for rows.Next() {
		var e model.WatchEvent
		var epID, movID sql.NullString
		if err := rows.Scan(&e.ID, &e.UserID, &epID, &movID, &e.WatchedAt, &e.Notes); err != nil {
			return nil, err
		}
		if epID.Valid {
			s := epID.String
			e.EpisodeID = &s
		}
		if movID.Valid {
			s := movID.String
			e.MovieID = &s
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
