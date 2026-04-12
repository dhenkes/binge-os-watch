package model

import "context"

// New-schema rating types. One per subject (show / movie / season /
// episode), each backed by a table with proper FKs that cascade on
// subject deletion. The old polymorphic Rating stays in rating.go for
// Phase B to phase out.

type RatingShow struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	ShowID    string `json:"show_id"`
	Score     int    `json:"score"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type RatingMovie struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	MovieID   string `json:"movie_id"`
	Score     int    `json:"score"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type RatingSeasonV2 struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	SeasonID  string `json:"season_id"`
	Score     int    `json:"score"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type RatingEpisodeV2 struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	EpisodeID string `json:"episode_id"`
	Score     int    `json:"score"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// RatingRepositoryV2 is the per-subject replacement for the polymorphic
// RatingRepository. Returns nil with no error when no rating exists, so
// callers can treat "unrated" as the zero value without sniffing errors.
type RatingRepositoryV2 interface {
	// Show
	UpsertShow(ctx context.Context, userID, showID string, score int) error
	GetShow(ctx context.Context, userID, showID string) (*RatingShow, error)
	DeleteShow(ctx context.Context, userID, showID string) error

	// Movie
	UpsertMovie(ctx context.Context, userID, movieID string, score int) error
	GetMovie(ctx context.Context, userID, movieID string) (*RatingMovie, error)
	DeleteMovie(ctx context.Context, userID, movieID string) error

	// Season
	UpsertSeason(ctx context.Context, userID, seasonID string, score int) error
	GetSeason(ctx context.Context, userID, seasonID string) (*RatingSeasonV2, error)
	DeleteSeason(ctx context.Context, userID, seasonID string) error

	// Episode
	UpsertEpisode(ctx context.Context, userID, episodeID string, score int) error
	GetEpisode(ctx context.Context, userID, episodeID string) (*RatingEpisodeV2, error)
	DeleteEpisode(ctx context.Context, userID, episodeID string) error

	// Batch helpers for detail-page rendering without N+1.
	ListShowRatingsByIDs(ctx context.Context, userID string, showIDs []string) (map[string]int, error)
	ListMovieRatingsByIDs(ctx context.Context, userID string, movieIDs []string) (map[string]int, error)
	ListSeasonRatingsByShow(ctx context.Context, userID, showID string) (map[string]int, error)
	ListEpisodeRatingsByShow(ctx context.Context, userID, showID string) (map[string]int, error)
}

// RatingServiceV2 owns business logic for scoring any subject. Zero score
// means "delete the rating" to mirror the existing UI affordance.
type RatingServiceV2 interface {
	RateShow(ctx context.Context, userID, showID string, score int) error
	RateMovie(ctx context.Context, userID, movieID string, score int) error
	RateSeason(ctx context.Context, userID, seasonID string, score int) error
	RateEpisode(ctx context.Context, userID, episodeID string, score int) error
}
