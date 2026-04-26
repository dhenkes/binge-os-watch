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

type TMDBEpisodeRepository struct {
	repo
}

var _ model.TMDBEpisodeRepository = (*TMDBEpisodeRepository)(nil)

func NewTMDBEpisodeRepository(db DBTX) *TMDBEpisodeRepository {
	return &TMDBEpisodeRepository{repo{db: db}}
}

// UpsertBatch inserts or updates episodes from TMDB. tmdb_episode has two
// UNIQUE constraints: tmdb_episode_id (the stable TMDB identity) and
// (season_id, episode_number). When TMDB reshuffles a season — two
// episodes swap numbers, an episode moves to another season, an episode
// disappears upstream while a new one takes its slot — updating rows one
// by one trips the second constraint mid-batch because the destination
// slot is still occupied by another row.
//
// Pass 1 parks every row that could be in the way at
// episode_number = -tmdb_episode_id (globally unique negatives, so they
// can neither collide with each other nor with the incoming positive
// numbers). Two flavors of "in the way":
//   - Rows whose tmdb_episode_id is in the batch — they'll be moved to a
//     new (season_id, episode_number) by pass 2.
//   - Rows currently sitting in a (season_id, episode_number) slot the
//     batch wants, even if their tmdb_episode_id isn't in the batch —
//     this is the "stale row" case (TMDB stopped reporting an episode
//     but a new one took its slot). We can't delete the stale row
//     because watch_event references it; parking preserves user history
//     while freeing the slot.
//
// Pass 2 runs the real upsert against a clean slot space. Caller wraps
// this in a transaction (refresh_catalog does via txFunc), so any
// partial failure rolls back cleanly — including the parked state.
func (r *TMDBEpisodeRepository) UpsertBatch(ctx context.Context, episodes []model.TMDBEpisode) error {
	if len(episodes) == 0 {
		return nil
	}

	// Pass 1: park.
	idPlaceholders := make([]string, len(episodes))
	slotPlaceholders := make([]string, len(episodes))
	parkArgs := make([]any, 0, len(episodes)*3)
	for i, e := range episodes {
		idPlaceholders[i] = "?"
		parkArgs = append(parkArgs, e.TMDBEpisodeID)
	}
	for i, e := range episodes {
		slotPlaceholders[i] = "(?, ?)"
		parkArgs = append(parkArgs, e.SeasonID, e.EpisodeNumber)
	}
	if _, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE tmdb_episode SET episode_number = -tmdb_episode_id
		 WHERE tmdb_episode_id IN (`+strings.Join(idPlaceholders, ",")+`)
		    OR (season_id, episode_number) IN (`+strings.Join(slotPlaceholders, ",")+`)`,
		parkArgs...,
	); err != nil {
		return fmt.Errorf("parking episodes: %w", err)
	}

	// Pass 2: real upsert.
	for i := range episodes {
		e := &episodes[i]
		if e.ID == "" {
			e.ID = uuid.NewString()
		}
		var existingID string
		err := r.conn(ctx).QueryRowContext(ctx,
			`SELECT id FROM tmdb_episode WHERE tmdb_episode_id = ?`,
			e.TMDBEpisodeID,
		).Scan(&existingID)
		switch {
		case err == nil:
			if _, err := r.conn(ctx).ExecContext(ctx,
				`UPDATE tmdb_episode
				 SET season_id = ?, episode_number = ?, name = ?, overview = ?,
				     still_path = ?, air_date = ?, runtime_minutes = ?
				 WHERE id = ?`,
				e.SeasonID, e.EpisodeNumber, e.Name, e.Overview,
				e.StillPath, e.AirDate, e.RuntimeMinutes, existingID,
			); err != nil {
				return fmt.Errorf("updating episode %d: %w", e.EpisodeNumber, err)
			}
			e.ID = existingID
		case errors.Is(err, sql.ErrNoRows):
			if _, err := r.conn(ctx).ExecContext(ctx,
				`INSERT INTO tmdb_episode (id, season_id, tmdb_episode_id, episode_number, name, overview, still_path, air_date, runtime_minutes)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
				 ON CONFLICT(season_id, episode_number) DO UPDATE SET
				     tmdb_episode_id = excluded.tmdb_episode_id,
				     name = excluded.name,
				     overview = excluded.overview,
				     still_path = excluded.still_path,
				     air_date = excluded.air_date,
				     runtime_minutes = excluded.runtime_minutes`,
				e.ID, e.SeasonID, e.TMDBEpisodeID, e.EpisodeNumber, e.Name, e.Overview,
				e.StillPath, e.AirDate, e.RuntimeMinutes,
			); err != nil {
				return fmt.Errorf("upserting episode %d: %w", e.EpisodeNumber, err)
			}
		default:
			return fmt.Errorf("looking up episode %d: %w", e.EpisodeNumber, err)
		}
	}
	return nil
}

func (r *TMDBEpisodeRepository) GetByID(ctx context.Context, id string) (*model.TMDBEpisode, error) {
	return scanTMDBEpisodeOne(r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, season_id, tmdb_episode_id, episode_number, name, overview, still_path, air_date, runtime_minutes
		 FROM tmdb_episode WHERE id = ?`, id))
}

func (r *TMDBEpisodeRepository) GetByTMDBID(ctx context.Context, tmdbEpisodeID int) (*model.TMDBEpisode, error) {
	return scanTMDBEpisodeOne(r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, season_id, tmdb_episode_id, episode_number, name, overview, still_path, air_date, runtime_minutes
		 FROM tmdb_episode WHERE tmdb_episode_id = ?`, tmdbEpisodeID))
}

func (r *TMDBEpisodeRepository) ListBySeason(ctx context.Context, seasonID string) ([]model.TMDBEpisode, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT id, season_id, tmdb_episode_id, episode_number, name, overview, still_path, air_date, runtime_minutes
		 FROM tmdb_episode WHERE season_id = ? ORDER BY episode_number`, seasonID)
	if err != nil {
		return nil, fmt.Errorf("listing episodes by season: %w", err)
	}
	defer rows.Close()
	var out []model.TMDBEpisode
	for rows.Next() {
		e, err := scanTMDBEpisode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

// ListByShow returns every episode of a show in (season_number, episode_number)
// order. Used by the media detail page to render the full episode list
// without per-season round trips.
func (r *TMDBEpisodeRepository) ListByShow(ctx context.Context, showID string) ([]model.TMDBEpisode, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT e.id, e.season_id, e.tmdb_episode_id, e.episode_number, e.name, e.overview, e.still_path, e.air_date, e.runtime_minutes
		 FROM tmdb_episode e
		 JOIN tmdb_season s ON e.season_id = s.id
		 WHERE s.show_id = ?
		 ORDER BY s.season_number, e.episode_number`, showID)
	if err != nil {
		return nil, fmt.Errorf("listing episodes by show: %w", err)
	}
	defer rows.Close()
	var out []model.TMDBEpisode
	for rows.Next() {
		e, err := scanTMDBEpisode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

// LatestAiredByShows returns the most recent aired episode air_date for
// each of the given shows, keyed by show id. Shows with no aired episodes
// are omitted from the map. Specials (season 0) are excluded so the date
// reflects the regular release schedule.
func (r *TMDBEpisodeRepository) LatestAiredByShows(ctx context.Context, showIDs []string) (map[string]int64, error) {
	out := map[string]int64{}
	if len(showIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(showIDs))
	args := make([]any, len(showIDs))
	for i, id := range showIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `SELECT s.show_id, MAX(e.air_date)
		FROM tmdb_episode e
		JOIN tmdb_season s ON e.season_id = s.id
		WHERE s.show_id IN (` + strings.Join(placeholders, ",") + `)
		  AND s.season_number > 0
		  AND e.air_date IS NOT NULL
		  AND e.air_date <= CAST(strftime('%s','now') AS INTEGER)
		GROUP BY s.show_id`
	rows, err := r.conn(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("latest aired by shows: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var showID string
		var ts sql.NullInt64
		if err := rows.Scan(&showID, &ts); err != nil {
			return nil, err
		}
		if ts.Valid {
			out[showID] = ts.Int64
		}
	}
	return out, rows.Err()
}

func scanTMDBEpisodeOne(row scannable) (*model.TMDBEpisode, error) {
	e, err := scanTMDBEpisode(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.NewNotFound("tmdb episode not found")
		}
		return nil, err
	}
	return e, nil
}

func scanTMDBEpisode(row scannable) (*model.TMDBEpisode, error) {
	var e model.TMDBEpisode
	var airDate sql.NullInt64
	err := row.Scan(
		&e.ID, &e.SeasonID, &e.TMDBEpisodeID, &e.EpisodeNumber, &e.Name, &e.Overview,
		&e.StillPath, &airDate, &e.RuntimeMinutes,
	)
	if err != nil {
		return nil, err
	}
	if airDate.Valid {
		v := airDate.Int64
		e.AirDate = &v
	}
	return &e, nil
}
