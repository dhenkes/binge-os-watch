package repository

import (
	"testing"
)

func TestBuildUpdateClauses(t *testing.T) {
	allowed := map[string]any{
		"title":  "New Title",
		"status": "watching",
	}
	sets, args := buildUpdateClauses([]string{"title", "status", "unknown"}, allowed)
	if len(sets) != 2 {
		t.Fatalf("got %d clauses, want 2", len(sets))
	}
	if len(args) != 2 {
		t.Fatalf("got %d args, want 2", len(args))
	}
	if sets[0] != "title = ?" {
		t.Errorf("sets[0] = %q, want 'title = ?'", sets[0])
	}
	if args[0] != "New Title" {
		t.Errorf("args[0] = %v, want New Title", args[0])
	}
}

func TestBuildUpdateClauses_Empty(t *testing.T) {
	sets, args := buildUpdateClauses(nil, map[string]any{"title": "x"})
	if len(sets) != 0 {
		t.Errorf("expected no clauses, got %d", len(sets))
	}
	if len(args) != 0 {
		t.Errorf("expected no args, got %d", len(args))
	}
}

func TestBuildUpdateClauses_NoMatch(t *testing.T) {
	sets, _ := buildUpdateClauses([]string{"unknown"}, map[string]any{"title": "x"})
	if len(sets) != 0 {
		t.Errorf("expected no clauses for unknown field, got %d", len(sets))
	}
}

func TestBuildUpdateClauses_TrimSpace(t *testing.T) {
	sets, _ := buildUpdateClauses([]string{" title "}, map[string]any{"title": "x"})
	if len(sets) != 1 {
		t.Errorf("expected 1 clause after trimming, got %d", len(sets))
	}
}

func TestOffsetFromToken(t *testing.T) {
	tests := []struct {
		token   string
		want    int
		wantErr bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"20", 20, false},
		{"abc", 0, true},
	}
	for _, tt := range tests {
		got, err := offsetFromToken(tt.token)
		if (err != nil) != tt.wantErr {
			t.Errorf("offsetFromToken(%q) error = %v, wantErr = %v", tt.token, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("offsetFromToken(%q) = %d, want %d", tt.token, got, tt.want)
		}
	}
}

func TestNextToken(t *testing.T) {
	tests := []struct {
		offset, pageSize, total int
		want                    string
	}{
		{0, 20, 50, "20"},
		{20, 20, 50, "40"},
		{40, 20, 50, ""},  // no more pages
		{0, 20, 20, ""},   // exactly one page
		{0, 20, 10, ""},   // less than one page
	}
	for _, tt := range tests {
		got := nextToken(tt.offset, tt.pageSize, tt.total)
		if got != tt.want {
			t.Errorf("nextToken(%d, %d, %d) = %q, want %q", tt.offset, tt.pageSize, tt.total, got, tt.want)
		}
	}
}
