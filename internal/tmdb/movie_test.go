package tmdb

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetMovie(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/movie/550" {
			t.Errorf("path = %q, want /movie/550", r.URL.Path)
		}
		json.NewEncoder(w).Encode(MovieDetails{
			ID:    550,
			Title: "Fight Club",
		})
	}))

	movie, err := c.GetMovie(context.Background(), 550)
	if err != nil {
		t.Fatalf("GetMovie() error: %v", err)
	}
	if movie.ID != 550 {
		t.Errorf("ID = %d, want 550", movie.ID)
	}
	if movie.Title != "Fight Club" {
		t.Errorf("Title = %q, want Fight Club", movie.Title)
	}
}

func TestGetMovie_NotFound(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"status_message":"not found"}`))
	}))

	_, err := c.GetMovie(context.Background(), 999999)
	if err == nil {
		t.Fatal("GetMovie() should error on 404")
	}
}

func TestGetMovieRecommendations(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/movie/550/recommendations" {
			t.Errorf("path = %q, want /movie/550/recommendations", r.URL.Path)
		}
		json.NewEncoder(w).Encode(RecommendationResponse{
			Results: []Recommendation{
				{ID: 680, Title: "Pulp Fiction"},
				{ID: 137, Title: "Groundhog Day"},
			},
		})
	}))

	resp, err := c.GetMovieRecommendations(context.Background(), 550)
	if err != nil {
		t.Fatalf("GetMovieRecommendations() error: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("got %d results, want 2", len(resp.Results))
	}
	if resp.Results[0].MediaType != "movie" {
		t.Errorf("MediaType = %q, want movie", resp.Results[0].MediaType)
	}
}

func TestGetMovieWatchProviders(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/movie/550/watch/providers" {
			t.Errorf("path = %q, want /movie/550/watch/providers", r.URL.Path)
		}
		json.NewEncoder(w).Encode(WatchProviderResponse{
			ID: 550,
			Results: map[string]WatchProviders{
				"NL": {
					Stream: []Provider{{ProviderID: 8, ProviderName: "Netflix"}},
					Link:   "https://www.justwatch.com/nl/movie/fight-club",
				},
				"US": {
					Rent: []Provider{{ProviderID: 2, ProviderName: "Apple TV"}},
				},
			},
		})
	}))

	providers, err := c.GetMovieWatchProviders(context.Background(), 550, "NL")
	if err != nil {
		t.Fatalf("GetMovieWatchProviders() error: %v", err)
	}
	if len(providers.Stream) != 1 {
		t.Fatalf("got %d stream providers, want 1", len(providers.Stream))
	}
	if providers.Stream[0].ProviderName != "Netflix" {
		t.Errorf("provider = %q, want Netflix", providers.Stream[0].ProviderName)
	}
	if providers.Link != "https://www.justwatch.com/nl/movie/fight-club" {
		t.Errorf("link = %q", providers.Link)
	}
}

func TestGetMovieWatchProviders_RegionNotFound(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(WatchProviderResponse{
			ID:      550,
			Results: map[string]WatchProviders{},
		})
	}))

	providers, err := c.GetMovieWatchProviders(context.Background(), 550, "XX")
	if err != nil {
		t.Fatalf("GetMovieWatchProviders() error: %v", err)
	}
	if providers.Stream != nil {
		t.Errorf("expected empty stream providers for unknown region")
	}
}
