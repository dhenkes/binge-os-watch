package model

import "context"

// LibraryTagRepository is the new-schema join between tags and library
// entries. Replaces the old TagRepository.AddToMedia/RemoveFromMedia/
// ListByMedia methods that keyed on the per-user media row's id; under
// the new schema tags join to user_library.id instead.
type LibraryTagRepository interface {
	Add(ctx context.Context, libraryID, tagID string) error
	Remove(ctx context.Context, libraryID, tagID string) error
	ListByLibrary(ctx context.Context, libraryID string) ([]Tag, error)
	ListByUser(ctx context.Context, userID string) (map[string][]Tag, error) // libraryID → tags
}
