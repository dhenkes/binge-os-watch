package model

import "context"

// DiscoveryService defines business logic for trending, popular, and recommendations.
type DiscoveryService interface {
	Trending(ctx context.Context) (*TrendingResult, error)
	Popular(ctx context.Context, mediaType MediaType, page int) (*PopularResult, error)
	Recommendations(ctx context.Context, userID string) ([]RecommendationItem, error)
	WatchProviders(ctx context.Context, mediaID, region string) (*WatchProviderResult, error)
	MediaRecommendations(ctx context.Context, mediaID string) ([]RecommendationItem, error)
}

// TrendingResult holds trending movies and TV shows.
type TrendingResult struct {
	Movies []DiscoverItem `json:"movies"`
	TV     []DiscoverItem `json:"tv"`
}

// PopularResult holds paginated popular items.
type PopularResult struct {
	Items      []DiscoverItem `json:"items"`
	Page       int            `json:"page"`
	TotalPages int            `json:"total_pages"`
}

// DiscoverItem is a TMDB item suitable for display in discovery feeds.
type DiscoverItem struct {
	TMDBID      int     `json:"tmdb_id"`
	MediaType   string  `json:"media_type"`
	Title       string  `json:"title"`
	Overview    string  `json:"overview"`
	PosterPath  string  `json:"poster_path"`
	ReleaseDate string  `json:"release_date"`
	VoteAverage float64 `json:"vote_average"`
	InLibrary   bool    `json:"in_library"`
	MediaID     string  `json:"media_id,omitempty"` // set when InLibrary is true
}

// RecommendationItem is a TMDB recommendation with frequency ranking.
type RecommendationItem struct {
	TMDBID      int     `json:"tmdb_id"`
	MediaType   string  `json:"media_type"`
	Title       string  `json:"title"`
	Overview    string  `json:"overview"`
	PosterPath  string  `json:"poster_path"`
	ReleaseDate string  `json:"release_date"`
	VoteAverage float64 `json:"vote_average"`
	Count       int     `json:"count"` // recommended by N favorites
	InLibrary   bool    `json:"in_library"`
	MediaID     string  `json:"media_id,omitempty"` // set when InLibrary is true
}

// DismissedRecommendationRepository tracks dismissed recommendations.
type DismissedRecommendationRepository interface {
	Dismiss(ctx context.Context, userID string, tmdbID int, mediaType string) error
	ListAll(ctx context.Context, userID string) (map[string]bool, error)
}

// WatchProviderResult holds streaming/rent/buy info for a media item.
type WatchProviderResult struct {
	Stream []ProviderInfo `json:"stream,omitempty"`
	Rent   []ProviderInfo `json:"rent,omitempty"`
	Buy    []ProviderInfo `json:"buy,omitempty"`
	Link   string         `json:"link,omitempty"` // JustWatch attribution URL
}

// ProviderInfo represents a single streaming/rent/buy provider.
type ProviderInfo struct {
	ProviderID   int    `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	LogoPath     string `json:"logo_path"`
}
