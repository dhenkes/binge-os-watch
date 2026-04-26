package model

import "context"

// WatchEvent is one viewing — of either an episode or a movie, never both.
// Multiple rows per (user, subject) are allowed and represent rewatches.
type WatchEvent struct {
	ID        string  `json:"id"`
	UserID    string  `json:"user_id"`
	EpisodeID *string `json:"episode_id,omitempty"`
	MovieID   *string `json:"movie_id,omitempty"`
	WatchedAt int64   `json:"watched_at"`
	Notes     string  `json:"notes,omitempty"`
}

// WatchEventRepository persists viewing events and exposes the aggregate
// queries the UI needs (is this watched, how many unwatched aired, etc).
type WatchEventRepository interface {
	Create(ctx context.Context, event *WatchEvent) error
	Delete(ctx context.Context, id string) error
	DeleteLatestForEpisode(ctx context.Context, userID, episodeID string) error
	DeleteAllForEpisode(ctx context.Context, userID, episodeID string) error
	DeleteLatestForMovie(ctx context.Context, userID, movieID string) error
	DeleteAllForMovie(ctx context.Context, userID, movieID string) error

	// Aggregates
	LatestForEpisode(ctx context.Context, userID, episodeID string) (*WatchEvent, error)
	LatestForMovie(ctx context.Context, userID, movieID string) (*WatchEvent, error)
	HasEpisode(ctx context.Context, userID, episodeID string) (bool, error)
	HasMovie(ctx context.Context, userID, movieID string) (bool, error)

	// For a single show: watched_count / total, next unwatched aired
	// regular episode, and a map of episode_id → latest watched_at for
	// bulk UI rendering without N+1.
	ProgressForShow(ctx context.Context, userID, showID string) (watched, total int, hasFuture bool, err error)
	NextUnwatched(ctx context.Context, userID, showID string) (*TMDBEpisode, error)
	WatchedMapForShow(ctx context.Context, userID, showID string) (map[string]int64, error)

	// Rewatch / history helpers
	ListForEpisode(ctx context.Context, userID, episodeID string) ([]WatchEvent, error)
	ListForMovie(ctx context.Context, userID, movieID string) ([]WatchEvent, error)

	// UpdateLatestNotesForEpisode updates the notes column on the most
	// recent watch_event row for (user, episode). Returns true if a row
	// was updated, false if none existed. Notes live on the watch_event
	// row under Option B — there is no separate notes column.
	UpdateLatestNotesForEpisode(ctx context.Context, userID, episodeID, notes string) (bool, error)
}

// WatchService owns the "mark watched" business logic. Every mutator
// refreshes the denormalized user_library.watched_at after the event is
// committed so list queries don't need a MAX subquery.
type WatchService interface {
	WatchEpisode(ctx context.Context, userID, episodeID string, watchedAt int64, notes string) error
	UnwatchEpisode(ctx context.Context, userID, episodeID string) error
	UnwatchAllForEpisode(ctx context.Context, userID, episodeID string) error
	WatchMovie(ctx context.Context, userID, movieID string, watchedAt int64, notes string) error
	UnwatchMovie(ctx context.Context, userID, movieID string) error
	WatchNext(ctx context.Context, userID, showID string) (*TMDBEpisode, error)
	WatchAllInSeason(ctx context.Context, userID, seasonID string) error
	WatchUpToEpisode(ctx context.Context, userID, episodeID string) error
	// WatchUpToEpisodeWithDate is the "bulk mark up to" variant that lets
	// the caller pick the per-event watched_at timestamp. mode is one of
	// "today", "release", or "custom". For "custom" mode, customAt is a
	// unix-seconds timestamp; for the other modes it is ignored.
	WatchUpToEpisodeWithDate(ctx context.Context, userID, episodeID, mode string, customAt int64) error
	UnwatchUpToEpisode(ctx context.Context, userID, episodeID string) error
	RecalcStatus(ctx context.Context, libraryID string) error
}
