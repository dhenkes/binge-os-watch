package model

import "context"

// LibraryEntry is one user's row for tracking a show or movie. Exactly one
// of ShowID / MovieID is set (see user_library CHECK). ManualStatus is nil
// unless the user has explicitly overridden the derived status.
//
// WatchedAt is denormalized — kept in sync with the most recent watch_event
// by the episode / movie services — so list queries stay cheap without a
// MAX subquery.
type LibraryEntry struct {
	ID              string  `json:"id"`
	UserID          string  `json:"user_id"`
	MediaType       MediaType `json:"media_type"`
	ShowID          *string `json:"show_id"`
	MovieID         *string `json:"movie_id"`
	ManualStatus    *MediaStatus `json:"manual_status"`
	WatchedAt       *int64  `json:"watched_at"`
	Notes           string  `json:"notes"`
	ReleaseNotified bool    `json:"release_notified"`
	CreatedAt       int64   `json:"created_at"`
	UpdatedAt       int64   `json:"updated_at"`
}

// LibraryView is a denormalized read-model that joins a LibraryEntry with
// its catalog row so handlers and templates can render without a second
// fetch. Populated by LibraryRepository.List and friends.
type LibraryView struct {
	Entry LibraryEntry `json:"entry"`
	Show  *TMDBShow    `json:"show,omitempty"`
	Movie *TMDBMovie   `json:"movie,omitempty"`

	// Derived/computed fields filled by the repository or service layer
	// for specific queries; not stored.
	Status         MediaStatus `json:"status"`                    // derived (or manual)
	UnwatchedCount int         `json:"unwatched_count,omitempty"`
	TotalEpisodes  int         `json:"total_episodes,omitempty"`
	WatchedCount   int         `json:"watched_count,omitempty"`
}

// Title returns the display title regardless of whether the entry is a
// show or a movie.
func (v LibraryView) Title() string {
	if v.Show != nil {
		return v.Show.Title
	}
	if v.Movie != nil {
		return v.Movie.Title
	}
	return ""
}

// LibraryFilter is the filter shape used by LibraryRepository.List.
type LibraryFilter struct {
	Status    MediaStatus   // logical status (matches derived OR manual)
	Statuses  []MediaStatus // OR filter
	MediaType MediaType
	TagID     string
	Query     string
	SortBy    string // "title", "created_at", "release_date", "watched_at", "unwatched"
	SortDir   string // "asc" or "desc"
}

// LibraryRepository persists per-user library entries and runs the
// derived-status / unwatched-count queries that used to live on the old
// MediaRepository.
type LibraryRepository interface {
	Create(ctx context.Context, entry *LibraryEntry) error
	GetByID(ctx context.Context, id string) (*LibraryView, error)
	GetByShow(ctx context.Context, userID, showID string) (*LibraryView, error)
	GetByMovie(ctx context.Context, userID, movieID string) (*LibraryView, error)
	GetByTMDBID(ctx context.Context, userID string, tmdbID int, mediaType MediaType) (*LibraryView, error)
	List(ctx context.Context, userID string, filter LibraryFilter, page PageRequest) (*PageResponse[LibraryView], error)
	ListContinueWatching(ctx context.Context, userID string, limit int) ([]LibraryView, error)
	ListUnratedWatched(ctx context.Context, userID string, limit int) ([]LibraryView, error)
	SetManualStatus(ctx context.Context, id string, status *MediaStatus) error
	UpdateWatchedAt(ctx context.Context, id string, watchedAt *int64) error
	UpdateNotes(ctx context.Context, id, notes string) error
	MarkReleaseNotified(ctx context.Context, id string) error
	ListPendingReleaseNotifications(ctx context.Context) ([]LibraryView, error)
	GetLibraryMap(ctx context.Context, userID string) (map[string]string, error)
	ListTopRatedEntries(ctx context.Context, userID string, limit int) ([]LibraryView, error)
	Delete(ctx context.Context, id string) error
	TotalCount(ctx context.Context, userID string) (int, error)
}

// AddStub carries the minimum metadata needed to stand up a placeholder
// catalog row synchronously, before the background TMDB fetch finishes.
// Callers that have search results in hand (search page, discover page)
// should populate as much as they know so the UI doesn't flash a
// "Loading…" title; anything left empty falls back to safe defaults.
type AddStub struct {
	Title       string
	Overview    string
	PosterPath  string
	ReleaseDate string // raw "YYYY-MM-DD" from TMDB
}

// LibraryService defines business logic for adding/removing library
// entries. Add creates the user_library row and a placeholder catalog
// row immediately; the full TMDB catalog (seasons, episodes, runtime,
// backdrops, etc.) is fetched by a background TMDBJob so the UI never
// blocks on TMDB.
type LibraryService interface {
	Add(ctx context.Context, userID string, tmdbID int, mediaType MediaType) (*LibraryView, error)
	AddWithStub(ctx context.Context, userID string, tmdbID int, mediaType MediaType, stub *AddStub) (*LibraryView, error)
	Remove(ctx context.Context, id string) error
	SetStatus(ctx context.Context, id string, status *MediaStatus) error
	UpdateNotes(ctx context.Context, id, notes string) error
	UpdateWatchedAt(ctx context.Context, id string, watchedAt *int64) error
	RefreshCatalog(ctx context.Context, tmdbShowID string) error
	RefreshAll(ctx context.Context) error
}
