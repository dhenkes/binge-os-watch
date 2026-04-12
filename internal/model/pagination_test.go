package model

import "testing"

func TestPageRequest_Normalize(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"Zero", 0, DefaultPageSize},
		{"Negative", -1, DefaultPageSize},
		{"Normal", 10, 10},
		{"AtMax", MaxPageSize, MaxPageSize},
		{"OverMax", MaxPageSize + 1, MaxPageSize},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := PageRequest{PageSize: tt.input}.Normalize()
			if p.PageSize != tt.expected {
				t.Errorf("got %d, want %d", p.PageSize, tt.expected)
			}
		})
	}
}

func TestPageRequest_Normalize_PreservesToken(t *testing.T) {
	p := PageRequest{PageSize: 0, PageToken: "abc123"}.Normalize()
	if p.PageToken != "abc123" {
		t.Errorf("Normalize should not change PageToken, got %q", p.PageToken)
	}
}

func TestPageResponse_EnsureItems_NilSlice(t *testing.T) {
	p := &PageResponse[string]{Items: nil}
	p.EnsureItems()
	if p.Items == nil {
		t.Error("EnsureItems should make Items non-nil")
	}
	if len(p.Items) != 0 {
		t.Errorf("expected empty slice, got %d items", len(p.Items))
	}
}

func TestPageResponse_EnsureItems_NonNilSlice(t *testing.T) {
	p := &PageResponse[string]{Items: []string{"a", "b"}}
	p.EnsureItems()
	if len(p.Items) != 2 {
		t.Errorf("EnsureItems should not change existing items, got %d", len(p.Items))
	}
}

func TestPageSizeConstants(t *testing.T) {
	if DefaultPageSize <= 0 {
		t.Errorf("DefaultPageSize should be positive, got %d", DefaultPageSize)
	}
	if MaxPageSize <= DefaultPageSize {
		t.Errorf("MaxPageSize (%d) should be greater than DefaultPageSize (%d)", MaxPageSize, DefaultPageSize)
	}
}
