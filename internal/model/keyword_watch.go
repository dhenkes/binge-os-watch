package model

import (
	"context"
	"strings"
	"time"
)

// KeywordResultStatus tracks whether a keyword match has been acted on.
type KeywordResultStatus string

const (
	KeywordResultPending   KeywordResultStatus = "pending"
	KeywordResultAdded     KeywordResultStatus = "added"
	KeywordResultDismissed KeywordResultStatus = "dismissed"
)

// KeywordWatch represents a saved search that runs daily against TMDB.
type KeywordWatch struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Keyword    string    `json:"keyword"`
	MediaTypes string    `json:"media_types"` // "movie", "tv", or "movie,tv"
	CreatedAt  time.Time `json:"created_at"`
}

// Validate checks keyword watch fields.
func (kw *KeywordWatch) Validate() error {
	v := NewValidationErrors()
	if strings.TrimSpace(kw.Keyword) == "" {
		v.Add("keyword", "must not be empty")
	}
	if kw.MediaTypes != "" {
		for _, t := range strings.Split(kw.MediaTypes, ",") {
			t = strings.TrimSpace(t)
			if t != "movie" && t != "tv" {
				v.Add("media_types", "each type must be 'movie' or 'tv'")
				break
			}
		}
	}
	return v.OrNil()
}

// KeywordResult represents a TMDB match for a keyword watch.
type KeywordResult struct {
	ID              string              `json:"id"`
	KeywordWatchID  string              `json:"keyword_watch_id"`
	TMDBID          int                 `json:"tmdb_id"`
	MediaType       MediaType           `json:"media_type"`
	Title           string              `json:"title"`
	PosterPath      string              `json:"poster_path"`
	ReleaseDate     string              `json:"release_date"`
	Status          KeywordResultStatus `json:"status"`
	CreatedAt       time.Time           `json:"created_at"`
	KeywordWatchIDs []string            `json:"keyword_watch_ids,omitempty"` // for dedup display
}

// KeywordWatchRepository defines persistence operations for keyword watches.
type KeywordWatchRepository interface {
	Create(ctx context.Context, kw *KeywordWatch) error
	GetByID(ctx context.Context, id string) (*KeywordWatch, error)
	ListAll(ctx context.Context) ([]KeywordWatch, error)
	ListByUser(ctx context.Context, userID string) ([]KeywordWatch, error)
	Update(ctx context.Context, kw *KeywordWatch, mask []string) error
	Delete(ctx context.Context, id string) error

	// Results
	CreateResult(ctx context.Context, result *KeywordResult) error
	ListPendingResults(ctx context.Context, userID string) ([]KeywordResult, error)
	ListDismissedResults(ctx context.Context, userID string) ([]KeywordResult, error)
	UpdateResultStatus(ctx context.Context, id string, status KeywordResultStatus) error
	DismissAllResults(ctx context.Context, keywordWatchID string) error
	PendingCount(ctx context.Context, userID string) (int, error)
	ResultExists(ctx context.Context, keywordWatchID string, tmdbID int, mediaType MediaType) (bool, error)
}

// KeywordWatchService defines business logic for keyword watches.
type KeywordWatchService interface {
	Create(ctx context.Context, userID, keyword, mediaTypes string) (*KeywordWatch, error)
	List(ctx context.Context, userID string) ([]KeywordWatch, error)
	Update(ctx context.Context, id string, mask []string, kw *KeywordWatch) error
	Delete(ctx context.Context, id string) error
	ListSuggestions(ctx context.Context, userID string) ([]KeywordResult, error)
	AddSuggestion(ctx context.Context, resultID string) error
	DismissSuggestion(ctx context.Context, resultID string) error
	DismissAll(ctx context.Context, keywordWatchID string) error
	PendingCount(ctx context.Context, userID string) (int, error)
	ListDismissed(ctx context.Context, userID string) ([]KeywordResult, error)
	RestoreSuggestion(ctx context.Context, resultID string) error
}
