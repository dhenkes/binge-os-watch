package service

import (
	"context"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// StatsServiceImpl implements model.StatsService.
type StatsServiceImpl struct {
	stats model.StatsRepository
}

var _ model.StatsService = (*StatsServiceImpl)(nil)

func NewStatsService(stats model.StatsRepository) *StatsServiceImpl {
	return &StatsServiceImpl{stats: stats}
}

func (s *StatsServiceImpl) GetUserStats(ctx context.Context, userID string) (*model.UserStats, error) {
	return s.stats.GetUserStats(ctx, userID)
}

func (s *StatsServiceImpl) GetGlobalStats(ctx context.Context) (*model.UserStats, error) {
	return s.stats.GetGlobalStats(ctx)
}

func (s *StatsServiceImpl) GetAdminStats(ctx context.Context) (*model.AdminStats, error) {
	return s.stats.GetAdminStats(ctx)
}

func (s *StatsServiceImpl) GetLastActiveByUser(ctx context.Context) (map[string]time.Time, error) {
	return s.stats.GetLastActiveByUser(ctx)
}
