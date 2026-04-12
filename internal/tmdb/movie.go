package tmdb

import (
	"context"
	"fmt"
)

// GetMovie fetches full movie details from TMDB.
// Endpoint: GET /movie/{id}
func (c *Client) GetMovie(ctx context.Context, tmdbID int) (*MovieDetails, error) {
	var result MovieDetails
	if err := c.get(ctx, fmt.Sprintf("/movie/%d", tmdbID), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetMovieRecommendations fetches recommendations for a movie.
// Endpoint: GET /movie/{id}/recommendations
func (c *Client) GetMovieRecommendations(ctx context.Context, tmdbID int) (*RecommendationResponse, error) {
	var result RecommendationResponse
	if err := c.get(ctx, fmt.Sprintf("/movie/%d/recommendations", tmdbID), nil, &result); err != nil {
		return nil, err
	}
	for i := range result.Results {
		if result.Results[i].MediaType == "" {
			result.Results[i].MediaType = "movie"
		}
	}
	return &result, nil
}

// GetMovieWatchProviders fetches streaming/rent/buy providers for a movie.
// Endpoint: GET /movie/{id}/watch/providers
func (c *Client) GetMovieWatchProviders(ctx context.Context, tmdbID int, region string) (*WatchProviders, error) {
	var raw WatchProviderResponse
	if err := c.get(ctx, fmt.Sprintf("/movie/%d/watch/providers", tmdbID), nil, &raw); err != nil {
		return nil, err
	}
	if providers, ok := raw.Results[region]; ok {
		return &providers, nil
	}
	return &WatchProviders{}, nil
}
