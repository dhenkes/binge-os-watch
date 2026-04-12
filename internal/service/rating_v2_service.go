package service

import (
	"context"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// RatingServiceV2Impl is the new-schema rating service. A zero score is
// interpreted as "delete the rating" to preserve the current UI flow
// where clicking the same star twice clears the rating.
type RatingServiceV2Impl struct {
	ratings model.RatingRepositoryV2
}

var _ model.RatingServiceV2 = (*RatingServiceV2Impl)(nil)

func NewRatingServiceV2(ratings model.RatingRepositoryV2) *RatingServiceV2Impl {
	return &RatingServiceV2Impl{ratings: ratings}
}

func (s *RatingServiceV2Impl) RateShow(ctx context.Context, userID, showID string, score int) error {
	if score <= 0 {
		return s.ratings.DeleteShow(ctx, userID, showID)
	}
	return s.ratings.UpsertShow(ctx, userID, showID, score)
}

func (s *RatingServiceV2Impl) RateMovie(ctx context.Context, userID, movieID string, score int) error {
	if score <= 0 {
		return s.ratings.DeleteMovie(ctx, userID, movieID)
	}
	return s.ratings.UpsertMovie(ctx, userID, movieID, score)
}

func (s *RatingServiceV2Impl) RateSeason(ctx context.Context, userID, seasonID string, score int) error {
	if score <= 0 {
		return s.ratings.DeleteSeason(ctx, userID, seasonID)
	}
	return s.ratings.UpsertSeason(ctx, userID, seasonID, score)
}

func (s *RatingServiceV2Impl) RateEpisode(ctx context.Context, userID, episodeID string, score int) error {
	if score <= 0 {
		return s.ratings.DeleteEpisode(ctx, userID, episodeID)
	}
	return s.ratings.UpsertEpisode(ctx, userID, episodeID, score)
}
