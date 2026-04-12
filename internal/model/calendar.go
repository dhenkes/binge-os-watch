package model

import "context"

// CalendarEntry represents a single item in the release calendar feed.
type CalendarEntry struct {
	MediaID     string    `json:"media_id"`
	MediaTitle  string    `json:"media_title"`
	MediaType   MediaType `json:"media_type"`
	PosterPath  string    `json:"poster_path"`
	ReleaseDate string    `json:"release_date"` // YYYY-MM-DD
	EpisodeInfo string    `json:"episode_info,omitempty"` // "S02E05 - Episode Title"
	EpisodeID   string    `json:"episode_id,omitempty"`
	Watched     bool      `json:"watched"`
}

// CalendarFilter holds query parameters for the calendar feed.
type CalendarFilter struct {
	MediaType MediaType   // "movie", "tv", or "" for both
	Status    MediaStatus // filter by media status (watching, plan_to_watch)
	Range     string      // "7d", "30d", "90d", "all"
	Section   string      // "upcoming" or "recent"
}

// CalendarRepository defines persistence operations for the calendar.
type CalendarRepository interface {
	Upcoming(ctx context.Context, userID string, filter CalendarFilter) ([]CalendarEntry, error)
	RecentlyReleased(ctx context.Context, userID string, filter CalendarFilter) ([]CalendarEntry, error)
}

// CalendarService defines business logic for the release calendar.
type CalendarService interface {
	Upcoming(ctx context.Context, userID string, filter CalendarFilter) ([]CalendarEntry, error)
	RecentlyReleased(ctx context.Context, userID string, filter CalendarFilter) ([]CalendarEntry, error)
}
