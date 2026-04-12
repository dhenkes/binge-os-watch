package service

import (
	"context"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

type KeywordWatchServiceImpl struct {
	watches model.KeywordWatchRepository
	library model.LibraryService
}

var _ model.KeywordWatchService = (*KeywordWatchServiceImpl)(nil)

func NewKeywordWatchService(watches model.KeywordWatchRepository, library model.LibraryService) *KeywordWatchServiceImpl {
	return &KeywordWatchServiceImpl{watches: watches, library: library}
}

func (s *KeywordWatchServiceImpl) Create(ctx context.Context, userID, keyword, mediaTypes string) (*model.KeywordWatch, error) {
	kw := &model.KeywordWatch{
		UserID:     userID,
		Keyword:    keyword,
		MediaTypes: mediaTypes,
	}
	if kw.MediaTypes == "" {
		kw.MediaTypes = "movie,tv"
	}
	if err := kw.Validate(); err != nil {
		return nil, err
	}
	if err := s.watches.Create(ctx, kw); err != nil {
		return nil, err
	}
	return kw, nil
}

func (s *KeywordWatchServiceImpl) List(ctx context.Context, userID string) ([]model.KeywordWatch, error) {
	return s.watches.ListByUser(ctx, userID)
}

func (s *KeywordWatchServiceImpl) Update(ctx context.Context, id string, mask []string, kw *model.KeywordWatch) error {
	existing, err := s.watches.GetByID(ctx, id)
	if err != nil {
		return err
	}
	kw.ID = existing.ID
	return s.watches.Update(ctx, kw, mask)
}

func (s *KeywordWatchServiceImpl) Delete(ctx context.Context, id string) error {
	return s.watches.Delete(ctx, id)
}

func (s *KeywordWatchServiceImpl) ListSuggestions(ctx context.Context, userID string) ([]model.KeywordResult, error) {
	return s.watches.ListPendingResults(ctx, userID)
}

func (s *KeywordWatchServiceImpl) AddSuggestion(ctx context.Context, resultID string) error {
	return s.watches.UpdateResultStatus(ctx, resultID, model.KeywordResultAdded)
}

func (s *KeywordWatchServiceImpl) DismissSuggestion(ctx context.Context, resultID string) error {
	return s.watches.UpdateResultStatus(ctx, resultID, model.KeywordResultDismissed)
}

func (s *KeywordWatchServiceImpl) DismissAll(ctx context.Context, keywordWatchID string) error {
	return s.watches.DismissAllResults(ctx, keywordWatchID)
}

func (s *KeywordWatchServiceImpl) PendingCount(ctx context.Context, userID string) (int, error) {
	return s.watches.PendingCount(ctx, userID)
}

func (s *KeywordWatchServiceImpl) ListDismissed(ctx context.Context, userID string) ([]model.KeywordResult, error) {
	return s.watches.ListDismissedResults(ctx, userID)
}

func (s *KeywordWatchServiceImpl) RestoreSuggestion(ctx context.Context, resultID string) error {
	return s.watches.UpdateResultStatus(ctx, resultID, model.KeywordResultPending)
}
