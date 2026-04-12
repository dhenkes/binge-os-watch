package tmdb

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestTrendingMovies(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/trending/movie/week" {
			t.Errorf("path = %q, want /trending/movie/week", r.URL.Path)
		}
		json.NewEncoder(w).Encode(TrendingResponse{
			Results: []SearchResult{
				{ID: 1, Title: "Trending Movie"},
			},
		})
	}))

	resp, err := c.TrendingMovies(context.Background())
	if err != nil {
		t.Fatalf("TrendingMovies() error: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(resp.Results))
	}
	if resp.Results[0].MediaType != "movie" {
		t.Errorf("MediaType = %q, want movie", resp.Results[0].MediaType)
	}
}

func TestTrendingTV(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/trending/tv/week" {
			t.Errorf("path = %q, want /trending/tv/week", r.URL.Path)
		}
		json.NewEncoder(w).Encode(TrendingResponse{
			Results: []SearchResult{
				{ID: 1, Name: "Trending Show"},
			},
		})
	}))

	resp, err := c.TrendingTV(context.Background())
	if err != nil {
		t.Fatalf("TrendingTV() error: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(resp.Results))
	}
	if resp.Results[0].MediaType != "tv" {
		t.Errorf("MediaType = %q, want tv", resp.Results[0].MediaType)
	}
}

func TestPopularMovies(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/movie/popular" {
			t.Errorf("path = %q, want /movie/popular", r.URL.Path)
		}
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("page = %q, want 2", r.URL.Query().Get("page"))
		}
		json.NewEncoder(w).Encode(SearchResponse{
			Page: 2,
			Results: []SearchResult{
				{ID: 100, Title: "Popular Movie"},
			},
		})
	}))

	resp, err := c.PopularMovies(context.Background(), 2)
	if err != nil {
		t.Fatalf("PopularMovies() error: %v", err)
	}
	if resp.Page != 2 {
		t.Errorf("Page = %d, want 2", resp.Page)
	}
	if resp.Results[0].MediaType != "movie" {
		t.Errorf("MediaType = %q, want movie", resp.Results[0].MediaType)
	}
}

func TestPopularTV(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tv/popular" {
			t.Errorf("path = %q, want /tv/popular", r.URL.Path)
		}
		json.NewEncoder(w).Encode(SearchResponse{
			Results: []SearchResult{
				{ID: 200, Name: "Popular Show"},
			},
		})
	}))

	resp, err := c.PopularTV(context.Background(), 1)
	if err != nil {
		t.Fatalf("PopularTV() error: %v", err)
	}
	if resp.Results[0].MediaType != "tv" {
		t.Errorf("MediaType = %q, want tv", resp.Results[0].MediaType)
	}
}

func TestTrendingMovies_ServerError(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))

	_, err := c.TrendingMovies(context.Background())
	if err == nil {
		t.Fatal("TrendingMovies() should error on 503")
	}
}
