package service

import (
	"context"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

type TagServiceImpl struct {
	tags model.TagRepository
}

var _ model.TagService = (*TagServiceImpl)(nil)

func NewTagService(tags model.TagRepository) *TagServiceImpl {
	return &TagServiceImpl{tags: tags}
}

func (s *TagServiceImpl) Create(ctx context.Context, userID, name string) (*model.Tag, error) {
	tag := &model.Tag{UserID: userID, Name: name}
	if err := tag.Validate(); err != nil {
		return nil, err
	}
	if err := s.tags.Create(ctx, tag); err != nil {
		return nil, err
	}
	return tag, nil
}

func (s *TagServiceImpl) List(ctx context.Context, userID string) ([]model.Tag, error) {
	return s.tags.ListByUser(ctx, userID)
}

func (s *TagServiceImpl) Delete(ctx context.Context, id string) error {
	return s.tags.Delete(ctx, id)
}
