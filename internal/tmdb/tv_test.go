package tmdb

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetTV(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tv/1399" {
			t.Errorf("path = %q, want /tv/1399", r.URL.Path)
		}
		json.NewEncoder(w).Encode(TVDetails{
			ID:               1399,
			Name:             "Breaking Bad",
			NumberOfSeasons:  5,
			NumberOfEpisodes: 62,
			Seasons: []TVSeason{
				{ID: 1, SeasonNumber: 1, EpisodeCount: 7},
				{ID: 2, SeasonNumber: 2, EpisodeCount: 13},
			},
		})
	}))

	tv, err := c.GetTV(context.Background(), 1399)
	if err != nil {
		t.Fatalf("GetTV() error: %v", err)
	}
	if tv.ID != 1399 {
		t.Errorf("ID = %d, want 1399", tv.ID)
	}
	if tv.Name != "Breaking Bad" {
		t.Errorf("Name = %q, want Breaking Bad", tv.Name)
	}
	if len(tv.Seasons) != 2 {
		t.Errorf("got %d seasons, want 2", len(tv.Seasons))
	}
}

func TestGetSeason(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tv/1399/season/1" {
			t.Errorf("path = %q, want /tv/1399/season/1", r.URL.Path)
		}
		json.NewEncoder(w).Encode(SeasonDetails{
			ID:           1,
			SeasonNumber: 1,
			Name:         "Season 1",
			Episodes: []TVEpisode{
				{ID: 1, EpisodeNumber: 1, Name: "Pilot", Runtime: 58},
				{ID: 2, EpisodeNumber: 2, Name: "Cat's in the Bag...", Runtime: 48},
			},
		})
	}))

	season, err := c.GetSeason(context.Background(), 1399, 1)
	if err != nil {
		t.Fatalf("GetSeason() error: %v", err)
	}
	if season.SeasonNumber != 1 {
		t.Errorf("SeasonNumber = %d, want 1", season.SeasonNumber)
	}
	if len(season.Episodes) != 2 {
		t.Fatalf("got %d episodes, want 2", len(season.Episodes))
	}
	if season.Episodes[0].Name != "Pilot" {
		t.Errorf("episode[0].Name = %q, want Pilot", season.Episodes[0].Name)
	}
}

func TestGetTVRecommendations(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tv/1399/recommendations" {
			t.Errorf("path = %q, want /tv/1399/recommendations", r.URL.Path)
		}
		json.NewEncoder(w).Encode(RecommendationResponse{
			Results: []Recommendation{
				{ID: 60059, Name: "Better Call Saul"},
			},
		})
	}))

	resp, err := c.GetTVRecommendations(context.Background(), 1399)
	if err != nil {
		t.Fatalf("GetTVRecommendations() error: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(resp.Results))
	}
	if resp.Results[0].MediaType != "tv" {
		t.Errorf("MediaType = %q, want tv", resp.Results[0].MediaType)
	}
}

func TestGetTVWatchProviders(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tv/1399/watch/providers" {
			t.Errorf("path = %q, want /tv/1399/watch/providers", r.URL.Path)
		}
		json.NewEncoder(w).Encode(WatchProviderResponse{
			ID: 1399,
			Results: map[string]WatchProviders{
				"US": {
					Stream: []Provider{{ProviderID: 337, ProviderName: "Disney Plus"}},
				},
			},
		})
	}))

	providers, err := c.GetTVWatchProviders(context.Background(), 1399, "US")
	if err != nil {
		t.Fatalf("GetTVWatchProviders() error: %v", err)
	}
	if len(providers.Stream) != 1 {
		t.Fatalf("got %d stream providers, want 1", len(providers.Stream))
	}
	if providers.Stream[0].ProviderName != "Disney Plus" {
		t.Errorf("provider = %q, want Disney Plus", providers.Stream[0].ProviderName)
	}
}

func TestGetTVWatchProviders_RegionNotFound(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(WatchProviderResponse{
			Results: map[string]WatchProviders{},
		})
	}))

	providers, err := c.GetTVWatchProviders(context.Background(), 1399, "XX")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if providers.Stream != nil {
		t.Error("expected empty providers for unknown region")
	}
}

func TestGetTV_NotFound(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	_, err := c.GetTV(context.Background(), 999999)
	if err == nil {
		t.Fatal("GetTV() should error on 404")
	}
}
