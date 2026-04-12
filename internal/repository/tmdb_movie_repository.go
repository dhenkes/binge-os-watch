package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/google/uuid"
)

type TMDBMovieRepository struct {
	repo
}

var _ model.TMDBMovieRepository = (*TMDBMovieRepository)(nil)

func NewTMDBMovieRepository(db DBTX) *TMDBMovieRepository {
	return &TMDBMovieRepository{repo{db: db}}
}

func (r *TMDBMovieRepository) Upsert(ctx context.Context, m *model.TMDBMovie) error {
	if m.ID == "" {
		existing, err := r.GetByTMDBID(ctx, m.TMDBID)
		if err == nil && existing != nil {
			m.ID = existing.ID
		} else {
			m.ID = uuid.NewString()
		}
	}
	_, err := r.conn(ctx).ExecContext(ctx,
		`INSERT INTO tmdb_movie (id, tmdb_id, title, overview, poster_path, backdrop_path, release_date, runtime_minutes, genres, tmdb_status, refreshed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(tmdb_id) DO UPDATE SET
		     title = excluded.title,
		     overview = excluded.overview,
		     poster_path = excluded.poster_path,
		     backdrop_path = excluded.backdrop_path,
		     release_date = excluded.release_date,
		     runtime_minutes = excluded.runtime_minutes,
		     genres = excluded.genres,
		     tmdb_status = excluded.tmdb_status,
		     refreshed_at = excluded.refreshed_at`,
		m.ID, m.TMDBID, m.Title, m.Overview, m.PosterPath, m.BackdropPath,
		m.ReleaseDate, m.RuntimeMinutes, m.Genres, m.TMDBStatus, m.RefreshedAt,
	)
	if err != nil {
		return fmt.Errorf("upserting tmdb_movie: %w", err)
	}
	return nil
}

func (r *TMDBMovieRepository) GetByID(ctx context.Context, id string) (*model.TMDBMovie, error) {
	return scanTMDBMovieOne(r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, tmdb_id, title, overview, poster_path, backdrop_path, release_date, runtime_minutes, genres, tmdb_status, refreshed_at
		 FROM tmdb_movie WHERE id = ?`, id))
}

func (r *TMDBMovieRepository) GetByTMDBID(ctx context.Context, tmdbID int) (*model.TMDBMovie, error) {
	return scanTMDBMovieOne(r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, tmdb_id, title, overview, poster_path, backdrop_path, release_date, runtime_minutes, genres, tmdb_status, refreshed_at
		 FROM tmdb_movie WHERE tmdb_id = ?`, tmdbID))
}

func scanTMDBMovieOne(row scannable) (*model.TMDBMovie, error) {
	var m model.TMDBMovie
	var releaseDate sql.NullInt64
	err := row.Scan(
		&m.ID, &m.TMDBID, &m.Title, &m.Overview, &m.PosterPath, &m.BackdropPath,
		&releaseDate, &m.RuntimeMinutes, &m.Genres, &m.TMDBStatus, &m.RefreshedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.NewNotFound("tmdb movie not found")
		}
		return nil, err
	}
	if releaseDate.Valid {
		v := releaseDate.Int64
		m.ReleaseDate = &v
	}
	return &m, nil
}
