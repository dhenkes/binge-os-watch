package model

import (
	"context"
	"time"
)

// UserStats holds computed statistics for a single user or globally.
type UserStats struct {
	TotalMovies      int
	TotalEpisodes    int
	TotalTimeMinutes int
	AvgRating        float64
}

// AdminStats holds admin-specific statistics.
type AdminStats struct {
	TotalUsers       int
	ActiveUsersDay   int
	ActiveUsersWeek  int
	ActiveUsersMonth int
	NewMediaDay      int
	NewMediaWeek     int
	NewMediaMonth    int
}

// StatsRepository defines aggregate query operations for statistics.
type StatsRepository interface {
	GetUserStats(ctx context.Context, userID string) (*UserStats, error)
	GetGlobalStats(ctx context.Context) (*UserStats, error)
	GetAdminStats(ctx context.Context) (*AdminStats, error)
	GetLastActiveByUser(ctx context.Context) (map[string]time.Time, error)
}

// StatsService defines business logic for statistics.
type StatsService interface {
	GetUserStats(ctx context.Context, userID string) (*UserStats, error)
	GetGlobalStats(ctx context.Context) (*UserStats, error)
	GetAdminStats(ctx context.Context) (*AdminStats, error)
	GetLastActiveByUser(ctx context.Context) (map[string]time.Time, error)
}
