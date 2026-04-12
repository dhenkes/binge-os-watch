package model

import (
	"context"
	"strings"
	"time"
)

// Tag represents a user-defined label for organizing library entries.
type Tag struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Validate checks tag fields.
func (t *Tag) Validate() error {
	v := NewValidationErrors()
	if strings.TrimSpace(t.Name) == "" {
		v.Add("name", "must not be empty")
	}
	return v.OrNil()
}

// TagRepository defines persistence operations for tag definitions. The
// join between tags and library entries lives in LibraryTagRepository
// (library_tag.go).
type TagRepository interface {
	Create(ctx context.Context, tag *Tag) error
	GetByID(ctx context.Context, id string) (*Tag, error)
	ListByUser(ctx context.Context, userID string) ([]Tag, error)
	Delete(ctx context.Context, id string) error
}

// TagService defines business logic for tags.
type TagService interface {
	Create(ctx context.Context, userID, name string) (*Tag, error)
	List(ctx context.Context, userID string) ([]Tag, error)
	Delete(ctx context.Context, id string) error
}
