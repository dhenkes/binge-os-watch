package model

import "context"

// TMDB catalog types. One row per TMDB entity, shared across every user in
// the system — metadata sync refreshes these once globally instead of once
// per user-who-tracks-it. All timestamps are unix seconds; NULL means
// unknown (unaired episode, no release date, etc.).

// TMDBShow is a TV show as known to the local catalog.
type TMDBShow struct {
	ID            string  `json:"id"`
	TMDBID        int     `json:"tmdb_id"`
	Title         string  `json:"title"`
	Overview      string  `json:"overview"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	FirstAirDate  *int64  `json:"first_air_date"` // unix seconds
	Genres        string  `json:"genres"`
	TMDBStatus    string  `json:"tmdb_status"`
	RefreshedAt   int64   `json:"refreshed_at"`
}

// TMDBMovie is a movie as known to the local catalog.
type TMDBMovie struct {
	ID             string `json:"id"`
	TMDBID         int    `json:"tmdb_id"`
	Title          string `json:"title"`
	Overview       string `json:"overview"`
	PosterPath     string `json:"poster_path"`
	BackdropPath   string `json:"backdrop_path"`
	ReleaseDate    *int64 `json:"release_date"` // unix seconds
	RuntimeMinutes int    `json:"runtime_minutes"`
	Genres         string `json:"genres"`
	TMDBStatus     string `json:"tmdb_status"`
	RefreshedAt    int64  `json:"refreshed_at"`
}

// TMDBSeason is one season of a TMDBShow.
type TMDBSeason struct {
	ID           string `json:"id"`
	ShowID       string `json:"show_id"`
	TMDBSeasonID int    `json:"tmdb_season_id"`
	SeasonNumber int    `json:"season_number"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	PosterPath   string `json:"poster_path"`
	AirDate      *int64 `json:"air_date"`
	EpisodeCount int    `json:"episode_count"`
}

// TMDBEpisode is one episode of a TMDBSeason.
type TMDBEpisode struct {
	ID             string `json:"id"`
	SeasonID       string `json:"season_id"`
	TMDBEpisodeID  int    `json:"tmdb_episode_id"`
	EpisodeNumber  int    `json:"episode_number"`
	Name           string `json:"name"`
	Overview       string `json:"overview"`
	StillPath      string `json:"still_path"`
	AirDate        *int64 `json:"air_date"`
	RuntimeMinutes int    `json:"runtime_minutes"`
}

// TMDBShowRepository persists the shared show catalog.
type TMDBShowRepository interface {
	Upsert(ctx context.Context, show *TMDBShow) error
	GetByID(ctx context.Context, id string) (*TMDBShow, error)
	GetByTMDBID(ctx context.Context, tmdbID int) (*TMDBShow, error)
	ListAll(ctx context.Context) ([]TMDBShow, error)
	ListByTerminalStatus(ctx context.Context, terminal bool) ([]TMDBShow, error)
}

// TMDBMovieRepository persists the shared movie catalog.
type TMDBMovieRepository interface {
	Upsert(ctx context.Context, movie *TMDBMovie) error
	GetByID(ctx context.Context, id string) (*TMDBMovie, error)
	GetByTMDBID(ctx context.Context, tmdbID int) (*TMDBMovie, error)
}

// TMDBSeasonRepository persists the shared season catalog.
type TMDBSeasonRepository interface {
	UpsertBatch(ctx context.Context, seasons []TMDBSeason) error
	GetByID(ctx context.Context, id string) (*TMDBSeason, error)
	ListByShow(ctx context.Context, showID string) ([]TMDBSeason, error)
}

// TMDBEpisodeRepository persists the shared episode catalog.
type TMDBEpisodeRepository interface {
	UpsertBatch(ctx context.Context, episodes []TMDBEpisode) error
	GetByID(ctx context.Context, id string) (*TMDBEpisode, error)
	GetByTMDBID(ctx context.Context, tmdbEpisodeID int) (*TMDBEpisode, error)
	ListBySeason(ctx context.Context, seasonID string) ([]TMDBEpisode, error)
	ListByShow(ctx context.Context, showID string) ([]TMDBEpisode, error)
	LatestAiredByShows(ctx context.Context, showIDs []string) (map[string]int64, error)
}
