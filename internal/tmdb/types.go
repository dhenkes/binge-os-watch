// Package tmdb wraps all TMDB v3 API calls. This is the side effect boundary
// for all external API access. Services call tmdb functions but never make
// HTTP requests directly. Rate limiting lives here.
package tmdb

// SearchResult represents a single result from TMDB /search/multi.
type SearchResult struct {
	ID           int     `json:"id"`
	MediaType    string  `json:"media_type"` // "movie" or "tv"
	Title        string  `json:"title"`      // movie title
	Name         string  `json:"name"`       // TV show name
	Overview     string  `json:"overview"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	ReleaseDate  string  `json:"release_date"`  // movie
	FirstAirDate string  `json:"first_air_date"` // TV
	VoteAverage  float64 `json:"vote_average"`
	VoteCount    int     `json:"vote_count"`
}

// DisplayTitle returns the title for display, handling the movie/tv naming difference.
func (r SearchResult) DisplayTitle() string {
	if r.Title != "" {
		return r.Title
	}
	return r.Name
}

// DisplayDate returns the release date for display.
func (r SearchResult) DisplayDate() string {
	if r.ReleaseDate != "" {
		return r.ReleaseDate
	}
	return r.FirstAirDate
}

// SearchResponse is the paginated response from TMDB search endpoints.
type SearchResponse struct {
	Page         int            `json:"page"`
	TotalPages   int            `json:"total_pages"`
	TotalResults int            `json:"total_results"`
	Results      []SearchResult `json:"results"`
}

// MovieDetails holds the full details for a movie from TMDB.
type MovieDetails struct {
	ID           int      `json:"id"`
	Title        string   `json:"title"`
	Overview     string   `json:"overview"`
	PosterPath   string   `json:"poster_path"`
	BackdropPath string   `json:"backdrop_path"`
	ReleaseDate  string   `json:"release_date"`
	Runtime      int      `json:"runtime"`
	VoteAverage  float64  `json:"vote_average"`
	Genres       []Genre  `json:"genres"`
	Status       string   `json:"status"` // "Released", "In Production", etc.
}

// TVDetails holds the full details for a TV show from TMDB.
type TVDetails struct {
	ID               int        `json:"id"`
	Name             string     `json:"name"`
	Overview         string     `json:"overview"`
	PosterPath       string     `json:"poster_path"`
	BackdropPath     string     `json:"backdrop_path"`
	FirstAirDate     string     `json:"first_air_date"`
	VoteAverage      float64    `json:"vote_average"`
	Genres           []Genre    `json:"genres"`
	Status           string     `json:"status"` // "Returning Series", "Ended", etc.
	NumberOfSeasons  int        `json:"number_of_seasons"`
	NumberOfEpisodes int        `json:"number_of_episodes"`
	Seasons          []TVSeason `json:"seasons"`
}

// TVSeason holds season info as returned within TVDetails.
type TVSeason struct {
	ID           int    `json:"id"`
	SeasonNumber int    `json:"season_number"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	PosterPath   string `json:"poster_path"`
	AirDate      string `json:"air_date"`
	EpisodeCount int    `json:"episode_count"`
}

// SeasonDetails holds the full details for a TV season from TMDB.
type SeasonDetails struct {
	ID           int         `json:"id"`
	SeasonNumber int         `json:"season_number"`
	Name         string      `json:"name"`
	Overview     string      `json:"overview"`
	PosterPath   string      `json:"poster_path"`
	AirDate      string      `json:"air_date"`
	Episodes     []TVEpisode `json:"episodes"`
}

// TVEpisode holds episode info as returned within SeasonDetails.
type TVEpisode struct {
	ID            int    `json:"id"`
	EpisodeNumber int    `json:"episode_number"`
	SeasonNumber  int    `json:"season_number"`
	Name          string `json:"name"`
	Overview      string `json:"overview"`
	StillPath     string `json:"still_path"`
	AirDate       string `json:"air_date"`
	Runtime       int    `json:"runtime"`
}

// Genre represents a TMDB genre.
type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// WatchProviders holds streaming/rent/buy info for a title in a given region.
type WatchProviders struct {
	Stream []Provider `json:"flatrate,omitempty"`
	Rent   []Provider `json:"rent,omitempty"`
	Buy    []Provider `json:"buy,omitempty"`
	Link   string     `json:"link"` // JustWatch attribution URL
}

// Provider represents a single streaming/rent/buy provider.
type Provider struct {
	ProviderID   int    `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	LogoPath     string `json:"logo_path"`
}

// WatchProviderResponse is the raw TMDB response for watch providers.
type WatchProviderResponse struct {
	ID      int                       `json:"id"`
	Results map[string]WatchProviders `json:"results"` // keyed by region code
}

// Recommendation represents a single TMDB recommendation.
type Recommendation struct {
	ID           int     `json:"id"`
	MediaType    string  `json:"media_type"`
	Title        string  `json:"title"`
	Name         string  `json:"name"`
	Overview     string  `json:"overview"`
	PosterPath   string  `json:"poster_path"`
	ReleaseDate  string  `json:"release_date"`
	FirstAirDate string  `json:"first_air_date"`
	VoteAverage  float64 `json:"vote_average"`
}

// DisplayTitle returns the title for display.
func (r Recommendation) DisplayTitle() string {
	if r.Title != "" {
		return r.Title
	}
	return r.Name
}

// RecommendationResponse is the paginated response from TMDB recommendation endpoints.
type RecommendationResponse struct {
	Page    int              `json:"page"`
	Results []Recommendation `json:"results"`
}

// TrendingResponse is the paginated response from TMDB trending endpoints.
type TrendingResponse struct {
	Page         int            `json:"page"`
	TotalPages   int            `json:"total_pages"`
	TotalResults int            `json:"total_results"`
	Results      []SearchResult `json:"results"`
}
