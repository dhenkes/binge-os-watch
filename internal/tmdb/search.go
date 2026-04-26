package tmdb

import (
	"context"
	"fmt"
	"net/url"
)

// SearchMulti searches TMDB for movies and TV shows matching the query.
// Endpoint: GET /search/multi?query={q}&page={page}
func (c *Client) SearchMulti(ctx context.Context, query string, page int) (*SearchResponse, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("include_adult", "false")

	var result SearchResponse
	if err := c.get(ctx, "/search/multi", params, &result); err != nil {
		return nil, err
	}

	// Filter to only movie and TV results (TMDB also returns "person").
	filtered := result.Results[:0]
	for _, r := range result.Results {
		if r.MediaType == "movie" || r.MediaType == "tv" {
			filtered = append(filtered, r)
		}
	}
	result.Results = filtered

	return &result, nil
}

// SearchMovies searches TMDB for movies matching the query. year=0 means
// no year filter; otherwise narrows to that primary release year.
// Endpoint: GET /search/movie?query={q}&page={page}&primary_release_year={year}
func (c *Client) SearchMovies(ctx context.Context, query string, page, year int) (*SearchResponse, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("page", fmt.Sprintf("%d", page))
	if year > 0 {
		params.Set("primary_release_year", fmt.Sprintf("%d", year))
	}

	var result SearchResponse
	if err := c.get(ctx, "/search/movie", params, &result); err != nil {
		return nil, err
	}

	for i := range result.Results {
		result.Results[i].MediaType = "movie"
	}

	return &result, nil
}

// SearchTV searches TMDB for TV shows matching the query. year=0 means no
// year filter; otherwise narrows to that first-air-date year.
// Endpoint: GET /search/tv?query={q}&page={page}&first_air_date_year={year}
func (c *Client) SearchTV(ctx context.Context, query string, page, year int) (*SearchResponse, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("page", fmt.Sprintf("%d", page))
	if year > 0 {
		params.Set("first_air_date_year", fmt.Sprintf("%d", year))
	}

	var result SearchResponse
	if err := c.get(ctx, "/search/tv", params, &result); err != nil {
		return nil, err
	}

	for i := range result.Results {
		result.Results[i].MediaType = "tv"
	}

	return &result, nil
}
