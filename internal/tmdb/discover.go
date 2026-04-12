package tmdb

import (
	"context"
	"fmt"
	"net/url"
)

// TrendingMovies fetches trending movies for the week.
// Endpoint: GET /trending/movie/week
func (c *Client) TrendingMovies(ctx context.Context) (*TrendingResponse, error) {
	var result TrendingResponse
	if err := c.get(ctx, "/trending/movie/week", nil, &result); err != nil {
		return nil, err
	}
	for i := range result.Results {
		result.Results[i].MediaType = "movie"
	}
	return &result, nil
}

// TrendingTV fetches trending TV shows for the week.
// Endpoint: GET /trending/tv/week
func (c *Client) TrendingTV(ctx context.Context) (*TrendingResponse, error) {
	var result TrendingResponse
	if err := c.get(ctx, "/trending/tv/week", nil, &result); err != nil {
		return nil, err
	}
	for i := range result.Results {
		result.Results[i].MediaType = "tv"
	}
	return &result, nil
}

// PopularMovies fetches popular movies with pagination.
// Endpoint: GET /movie/popular?page={page}
func (c *Client) PopularMovies(ctx context.Context, page int) (*SearchResponse, error) {
	params := url.Values{}
	params.Set("page", fmt.Sprintf("%d", page))

	var result SearchResponse
	if err := c.get(ctx, "/movie/popular", params, &result); err != nil {
		return nil, err
	}
	for i := range result.Results {
		result.Results[i].MediaType = "movie"
	}
	return &result, nil
}

// PopularTV fetches popular TV shows with pagination.
// Endpoint: GET /tv/popular?page={page}
func (c *Client) PopularTV(ctx context.Context, page int) (*SearchResponse, error) {
	params := url.Values{}
	params.Set("page", fmt.Sprintf("%d", page))

	var result SearchResponse
	if err := c.get(ctx, "/tv/popular", params, &result); err != nil {
		return nil, err
	}
	for i := range result.Results {
		result.Results[i].MediaType = "tv"
	}
	return &result, nil
}
