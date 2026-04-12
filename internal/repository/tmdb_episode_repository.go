package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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

func (r *TMDBEpisodeRepository) UpsertBatch(ctx context.Context, episodes []model.TMDBEpisode) error {
	for i := range episodes {
		e := &episodes[i]
		if e.ID == "" {
			e.ID = uuid.NewString()
		}
		_, err := r.conn(ctx).ExecContext(ctx,
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
		)
		if err != nil {
			return fmt.Errorf("upserting episode %d: %w", e.EpisodeNumber, err)
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
