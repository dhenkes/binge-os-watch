package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/google/uuid"
)

type TMDBSeasonRepository struct {
	repo
}

var _ model.TMDBSeasonRepository = (*TMDBSeasonRepository)(nil)

func NewTMDBSeasonRepository(db DBTX) *TMDBSeasonRepository {
	return &TMDBSeasonRepository{repo{db: db}}
}

// UpsertBatch upserts the given seasons, reusing existing IDs where the
// (show_id, season_number) pair already exists so foreign keys stay stable.
func (r *TMDBSeasonRepository) UpsertBatch(ctx context.Context, seasons []model.TMDBSeason) error {
	for i := range seasons {
		s := &seasons[i]
		if s.ID == "" {
			s.ID = uuid.NewString()
		}
		_, err := r.conn(ctx).ExecContext(ctx,
			`INSERT INTO tmdb_season (id, show_id, tmdb_season_id, season_number, name, overview, poster_path, air_date, episode_count)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(show_id, season_number) DO UPDATE SET
			     tmdb_season_id = excluded.tmdb_season_id,
			     name = excluded.name,
			     overview = excluded.overview,
			     poster_path = excluded.poster_path,
			     air_date = excluded.air_date,
			     episode_count = excluded.episode_count`,
			s.ID, s.ShowID, s.TMDBSeasonID, s.SeasonNumber, s.Name, s.Overview,
			s.PosterPath, s.AirDate, s.EpisodeCount,
		)
		if err != nil {
			return fmt.Errorf("upserting season %d: %w", s.SeasonNumber, err)
		}
	}
	return nil
}

func (r *TMDBSeasonRepository) GetByID(ctx context.Context, id string) (*model.TMDBSeason, error) {
	row := r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, show_id, tmdb_season_id, season_number, name, overview, poster_path, air_date, episode_count
		 FROM tmdb_season WHERE id = ?`, id)
	return scanTMDBSeasonOne(row)
}

func (r *TMDBSeasonRepository) ListByShow(ctx context.Context, showID string) ([]model.TMDBSeason, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT id, show_id, tmdb_season_id, season_number, name, overview, poster_path, air_date, episode_count
		 FROM tmdb_season WHERE show_id = ? ORDER BY season_number`, showID)
	if err != nil {
		return nil, fmt.Errorf("listing seasons by show: %w", err)
	}
	defer rows.Close()
	var out []model.TMDBSeason
	for rows.Next() {
		s, err := scanTMDBSeason(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func scanTMDBSeasonOne(row scannable) (*model.TMDBSeason, error) {
	s, err := scanTMDBSeason(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.NewNotFound("tmdb season not found")
		}
		return nil, err
	}
	return s, nil
}

func scanTMDBSeason(row scannable) (*model.TMDBSeason, error) {
	var s model.TMDBSeason
	var airDate sql.NullInt64
	err := row.Scan(
		&s.ID, &s.ShowID, &s.TMDBSeasonID, &s.SeasonNumber, &s.Name, &s.Overview,
		&s.PosterPath, &airDate, &s.EpisodeCount,
	)
	if err != nil {
		return nil, err
	}
	if airDate.Valid {
		v := airDate.Int64
		s.AirDate = &v
	}
	return &s, nil
}
