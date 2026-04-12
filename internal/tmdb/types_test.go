package tmdb

import "testing"

func TestSearchResult_DisplayTitle(t *testing.T) {
	tests := []struct {
		name   string
		result SearchResult
		want   string
	}{
		{"Movie", SearchResult{Title: "Fight Club", Name: ""}, "Fight Club"},
		{"TV", SearchResult{Title: "", Name: "Breaking Bad"}, "Breaking Bad"},
		{"Both", SearchResult{Title: "Movie Title", Name: "Show Name"}, "Movie Title"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.DisplayTitle(); got != tt.want {
				t.Errorf("DisplayTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSearchResult_DisplayDate(t *testing.T) {
	tests := []struct {
		name   string
		result SearchResult
		want   string
	}{
		{"Movie", SearchResult{ReleaseDate: "1999-10-15"}, "1999-10-15"},
		{"TV", SearchResult{FirstAirDate: "2008-01-20"}, "2008-01-20"},
		{"Both", SearchResult{ReleaseDate: "2020-01-01", FirstAirDate: "2021-01-01"}, "2020-01-01"},
		{"Neither", SearchResult{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.DisplayDate(); got != tt.want {
				t.Errorf("DisplayDate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRecommendation_DisplayTitle(t *testing.T) {
	r := Recommendation{Title: "Movie", Name: "Show"}
	if r.DisplayTitle() != "Movie" {
		t.Errorf("got %q, want Movie", r.DisplayTitle())
	}

	r = Recommendation{Name: "Show Only"}
	if r.DisplayTitle() != "Show Only" {
		t.Errorf("got %q, want Show Only", r.DisplayTitle())
	}
}
