package model

import "context"

// ImageService defines operations for cached images.
type ImageService interface {
	Get(ctx context.Context, sourcePath string) (*CachedImage, error)
}

// CachedImage holds image data for serving.
type CachedImage struct {
	Data        []byte
	ContentType string
}

// ICSService defines operations for ICS calendar feeds.
type ICSService interface {
	GenerateFeed(ctx context.Context, userID string) (string, error)
}

// TraktService defines operations for Trakt imports.
type TraktService interface {
	Import(ctx context.Context, userID string, data []byte) (added int, skipped int, err error)
}
