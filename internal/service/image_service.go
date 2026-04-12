package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// ImageService handles the cache-aside pattern for TMDB images using disk storage.
type ImageService struct {
	cacheDir string
	client   *http.Client
}

func NewImageService(cacheDir string) *ImageService {
	os.MkdirAll(cacheDir, 0755)
	return &ImageService{
		cacheDir: cacheDir,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Get returns a cached image from disk or fetches it from the TMDB CDN.
func (s *ImageService) Get(ctx context.Context, sourcePath string) (*model.CachedImage, error) {
	filePath := s.filePath(sourcePath)
	contentType := detectContentType(sourcePath)

	// Check disk cache.
	data, err := os.ReadFile(filePath)
	if err == nil {
		return &model.CachedImage{Data: data, ContentType: contentType}, nil
	}

	// Fetch from TMDB CDN.
	url := fmt.Sprintf("https://image.tmdb.org/t/p/%s", sourcePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating image request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image not found on TMDB CDN (status %d)", resp.StatusCode)
	}

	data, err = io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // 5MB max
	if err != nil {
		return nil, fmt.Errorf("reading image: %w", err)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		contentType = ct
	}

	// Write to disk cache.
	os.MkdirAll(filepath.Dir(filePath), 0755)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		// Log but don't fail — serve the image anyway.
		fmt.Printf("warning: failed to cache image %s: %v\n", filePath, err)
	}

	return &model.CachedImage{Data: data, ContentType: contentType}, nil
}

func (s *ImageService) filePath(sourcePath string) string {
	hash := sha256.Sum256([]byte(sourcePath))
	ext := ".jpg"
	lower := strings.ToLower(sourcePath)
	switch {
	case strings.HasSuffix(lower, ".png"):
		ext = ".png"
	case strings.HasSuffix(lower, ".webp"):
		ext = ".webp"
	}
	return filepath.Join(s.cacheDir, fmt.Sprintf("%x%s", hash, ext))
}

func detectContentType(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	default:
		return "image/jpeg"
	}
}
