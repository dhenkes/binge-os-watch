package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestImageService_GetFromCache(t *testing.T) {
	dir := t.TempDir()
	svc := NewImageService(dir)
	ctx := context.Background()

	// Pre-populate disk cache.
	filePath := svc.filePath("w342/cached.jpg")
	os.MkdirAll(filepath.Dir(filePath), 0755)
	os.WriteFile(filePath, []byte("cached data"), 0644)

	img, err := svc.Get(ctx, "w342/cached.jpg")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if string(img.Data) != "cached data" {
		t.Errorf("Data = %q, want cached data", img.Data)
	}
}

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"w342/abc.jpg", "image/jpeg"},
		{"w342/abc.png", "image/png"},
		{"w342/abc.webp", "image/webp"},
		{"w342/abc", "image/jpeg"},
	}
	for _, tt := range tests {
		got := detectContentType(tt.path)
		if got != tt.want {
			t.Errorf("detectContentType(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
