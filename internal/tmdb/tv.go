package tmdb

import (
	"context"
	"fmt"
)

// GetTV fetches full TV show details from TMDB (includes season summaries).
// Endpoint: GET /tv/{id}
func (c *Client) GetTV(ctx context.Context, tmdbID int) (*TVDetails, error) {
	var result TVDetails
	if err := c.get(ctx, fmt.Sprintf("/tv/%d", tmdbID), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSeason fetches full season details including all episodes.
// Endpoint: GET /tv/{tvID}/season/{seasonNumber}
func (c *Client) GetSeason(ctx context.Context, tvID, seasonNumber int) (*SeasonDetails, error) {
	var result SeasonDetails
	if err := c.get(ctx, fmt.Sprintf("/tv/%d/season/%d", tvID, seasonNumber), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTVRecommendations fetches recommendations for a TV show.
// Endpoint: GET /tv/{id}/recommendations
func (c *Client) GetTVRecommendations(ctx context.Context, tmdbID int) (*RecommendationResponse, error) {
	var result RecommendationResponse
	if err := c.get(ctx, fmt.Sprintf("/tv/%d/recommendations", tmdbID), nil, &result); err != nil {
		return nil, err
	}
	for i := range result.Results {
		if result.Results[i].MediaType == "" {
			result.Results[i].MediaType = "tv"
		}
	}
	return &result, nil
}

// GetTVWatchProviders fetches streaming/rent/buy providers for a TV show.
// Endpoint: GET /tv/{id}/watch/providers
func (c *Client) GetTVWatchProviders(ctx context.Context, tmdbID int, region string) (*WatchProviders, error) {
	var raw WatchProviderResponse
	if err := c.get(ctx, fmt.Sprintf("/tv/%d/watch/providers", tmdbID), nil, &raw); err != nil {
		return nil, err
	}
	if providers, ok := raw.Results[region]; ok {
		return &providers, nil
	}
	return &WatchProviders{}, nil
}
