package tmdb

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestSearchMulti(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/multi" {
			t.Errorf("path = %q, want /search/multi", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "batman" {
			t.Errorf("query = %q, want batman", r.URL.Query().Get("query"))
		}
		if r.URL.Query().Get("include_adult") != "false" {
			t.Errorf("include_adult = %q, want false", r.URL.Query().Get("include_adult"))
		}
		json.NewEncoder(w).Encode(SearchResponse{
			Page:         1,
			TotalResults: 3,
			Results: []SearchResult{
				{ID: 1, MediaType: "movie", Title: "Batman"},
				{ID: 2, MediaType: "tv", Name: "Batman Series"},
				{ID: 3, MediaType: "person", Name: "Ben Affleck"},
			},
		})
	}))

	resp, err := c.SearchMulti(context.Background(), "batman", 1)
	if err != nil {
		t.Fatalf("SearchMulti() error: %v", err)
	}
	// "person" results should be filtered out.
	if len(resp.Results) != 2 {
		t.Fatalf("got %d results, want 2 (person filtered)", len(resp.Results))
	}
	if resp.Results[0].MediaType != "movie" {
		t.Errorf("result[0].MediaType = %q, want movie", resp.Results[0].MediaType)
	}
	if resp.Results[1].MediaType != "tv" {
		t.Errorf("result[1].MediaType = %q, want tv", resp.Results[1].MediaType)
	}
}

func TestSearchMulti_Empty(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(SearchResponse{Results: []SearchResult{}})
	}))

	resp, err := c.SearchMulti(context.Background(), "xyznonexistent", 1)
	if err != nil {
		t.Fatalf("SearchMulti() error: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Errorf("got %d results, want 0", len(resp.Results))
	}
}

func TestSearchMovies(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/movie" {
			t.Errorf("path = %q, want /search/movie", r.URL.Path)
		}
		json.NewEncoder(w).Encode(SearchResponse{
			Results: []SearchResult{
				{ID: 550, Title: "Fight Club"},
			},
		})
	}))

	resp, err := c.SearchMovies(context.Background(), "fight club", 1, 0)
	if err != nil {
		t.Fatalf("SearchMovies() error: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(resp.Results))
	}
	if resp.Results[0].MediaType != "movie" {
		t.Errorf("MediaType = %q, want movie (should be set by client)", resp.Results[0].MediaType)
	}
}

func TestSearchTV(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/tv" {
			t.Errorf("path = %q, want /search/tv", r.URL.Path)
		}
		json.NewEncoder(w).Encode(SearchResponse{
			Results: []SearchResult{
				{ID: 1399, Name: "Breaking Bad"},
			},
		})
	}))

	resp, err := c.SearchTV(context.Background(), "breaking bad", 1, 0)
	if err != nil {
		t.Fatalf("SearchTV() error: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(resp.Results))
	}
	if resp.Results[0].MediaType != "tv" {
		t.Errorf("MediaType = %q, want tv", resp.Results[0].MediaType)
	}
}

func TestSearchMulti_ServerError(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	_, err := c.SearchMulti(context.Background(), "test", 1)
	if err == nil {
		t.Fatal("SearchMulti() should error on 500")
	}
}
