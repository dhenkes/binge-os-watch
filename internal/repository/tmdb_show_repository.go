package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/google/uuid"
)

// TMDBShowRepository is the shared show catalog — one row per TMDB show
// across the whole installation.
type TMDBShowRepository struct {
	repo
}

var _ model.TMDBShowRepository = (*TMDBShowRepository)(nil)

func NewTMDBShowRepository(db DBTX) *TMDBShowRepository {
	return &TMDBShowRepository{repo{db: db}}
}

// Upsert creates the show if it doesn't exist, otherwise refreshes the
// mutable columns. A new UUID is only allocated on first insert — existing
// rows keep theirs so foreign keys stay stable.
func (r *TMDBShowRepository) Upsert(ctx context.Context, s *model.TMDBShow) error {
	if s.ID == "" {
		existing, err := r.GetByTMDBID(ctx, s.TMDBID)
		if err == nil && existing != nil {
			s.ID = existing.ID
		} else {
			s.ID = uuid.NewString()
		}
	}

	_, err := r.conn(ctx).ExecContext(ctx,
		`INSERT INTO tmdb_show (id, tmdb_id, title, overview, poster_path, backdrop_path, first_air_date, genres, tmdb_status, refreshed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(tmdb_id) DO UPDATE SET
		     title = excluded.title,
		     overview = excluded.overview,
		     poster_path = excluded.poster_path,
		     backdrop_path = excluded.backdrop_path,
		     first_air_date = excluded.first_air_date,
		     genres = excluded.genres,
		     tmdb_status = excluded.tmdb_status,
		     refreshed_at = excluded.refreshed_at`,
		s.ID, s.TMDBID, s.Title, s.Overview, s.PosterPath, s.BackdropPath,
		s.FirstAirDate, s.Genres, s.TMDBStatus, s.RefreshedAt,
	)
	if err != nil {
		return fmt.Errorf("upserting tmdb_show: %w", err)
	}
	return nil
}

func (r *TMDBShowRepository) GetByID(ctx context.Context, id string) (*model.TMDBShow, error) {
	return r.scanOne(r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, tmdb_id, title, overview, poster_path, backdrop_path, first_air_date, genres, tmdb_status, refreshed_at
		 FROM tmdb_show WHERE id = ?`, id))
}

func (r *TMDBShowRepository) GetByTMDBID(ctx context.Context, tmdbID int) (*model.TMDBShow, error) {
	return r.scanOne(r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, tmdb_id, title, overview, poster_path, backdrop_path, first_air_date, genres, tmdb_status, refreshed_at
		 FROM tmdb_show WHERE tmdb_id = ?`, tmdbID))
}

func (r *TMDBShowRepository) ListAll(ctx context.Context) ([]model.TMDBShow, error) {
	return r.query(ctx,
		`SELECT id, tmdb_id, title, overview, poster_path, backdrop_path, first_air_date, genres, tmdb_status, refreshed_at
		 FROM tmdb_show ORDER BY title`)
}

// ListByTerminalStatus returns shows whose TMDB status is (or is not)
// terminal. Metadata sync uses it to skip ended/cancelled shows entirely.
func (r *TMDBShowRepository) ListByTerminalStatus(ctx context.Context, terminal bool) ([]model.TMDBShow, error) {
	if terminal {
		return r.query(ctx,
			`SELECT id, tmdb_id, title, overview, poster_path, backdrop_path, first_air_date, genres, tmdb_status, refreshed_at
			 FROM tmdb_show
			 WHERE tmdb_status IN ('Ended', 'Canceled', 'Cancelled')
			 ORDER BY refreshed_at ASC`)
	}
	return r.query(ctx,
		`SELECT id, tmdb_id, title, overview, poster_path, backdrop_path, first_air_date, genres, tmdb_status, refreshed_at
		 FROM tmdb_show
		 WHERE tmdb_status NOT IN ('Ended', 'Canceled', 'Cancelled')
		 ORDER BY refreshed_at ASC`)
}

func (r *TMDBShowRepository) query(ctx context.Context, sqlStr string, args ...any) ([]model.TMDBShow, error) {
	rows, err := r.conn(ctx).QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("querying tmdb_show: %w", err)
	}
	defer rows.Close()

	var out []model.TMDBShow
	for rows.Next() {
		s, err := scanTMDBShow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func (r *TMDBShowRepository) scanOne(row scannable) (*model.TMDBShow, error) {
	s, err := scanTMDBShow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.NewNotFound("tmdb show not found")
		}
		return nil, err
	}
	return s, nil
}

func scanTMDBShow(row scannable) (*model.TMDBShow, error) {
	var s model.TMDBShow
	var firstAirDate sql.NullInt64
	err := row.Scan(
		&s.ID, &s.TMDBID, &s.Title, &s.Overview, &s.PosterPath, &s.BackdropPath,
		&firstAirDate, &s.Genres, &s.TMDBStatus, &s.RefreshedAt,
	)
	if err != nil {
		return nil, err
	}
	if firstAirDate.Valid {
		v := firstAirDate.Int64
		s.FirstAirDate = &v
	}
	return &s, nil
}
