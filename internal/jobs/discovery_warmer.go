package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// DiscoveryWarmer keeps the discovery service's in-memory caches hot so
// the first user request after a cache miss doesn't pay the TMDB tax.
// The service's own caches still do the heavy lifting — this job just
// refreshes them on a timer a little before they'd expire.
//
// Trending has a 6h TTL in the service; we re-warm every 5h so a human
// browsing the discover tab never hits the cold path. Per-user
// recommendations have a 24h TTL but cost up to 10 sequential TMDB calls
// per user on cache miss, so we iterate every user and pre-warm once per
// warmInterval.
type DiscoveryWarmer struct {
	discovery model.DiscoveryService
	users     model.UserRepository
	interval  time.Duration
}

func NewDiscoveryWarmer(discovery model.DiscoveryService, users model.UserRepository, interval time.Duration) *DiscoveryWarmer {
	if interval <= 0 {
		interval = 5 * time.Hour
	}
	return &DiscoveryWarmer{discovery: discovery, users: users, interval: interval}
}

// Run starts the warmer loop and blocks until ctx is cancelled. Fires
// one warm pass immediately on startup so a fresh server boot doesn't
// serve a cold discover page either.
func (j *DiscoveryWarmer) Run(ctx context.Context) {
	slog.Info("discovery warmer started", "interval", j.interval)

	// Initial warm after a short delay so we don't slam TMDB during
	// normal startup, but still beat most real traffic.
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-timer.C:
			j.warm(ctx)
		case <-ticker.C:
			j.warm(ctx)
		case <-ctx.Done():
			slog.Info("discovery warmer stopped")
			return
		}
	}
}

func (j *DiscoveryWarmer) warm(ctx context.Context) {
	// Trending first — cheapest and hits every user.
	if _, err := j.discovery.Trending(ctx); err != nil {
		slog.Warn("discovery warmer: trending", "error", err)
	}

	// Per-user recommendations. If there are many users this could take
	// a while; each user is up to 10 sequential TMDB calls on cache
	// miss. That's fine in the background — we're not blocking any
	// request here.
	users, err := j.users.ListAll(ctx)
	if err != nil {
		slog.Warn("discovery warmer: list users", "error", err)
		return
	}
	for _, u := range users {
		if ctx.Err() != nil {
			return
		}
		if _, err := j.discovery.Recommendations(ctx, u.ID); err != nil {
			slog.Warn("discovery warmer: recs", "user_id", u.ID, "error", err)
		}
	}
	slog.Info("discovery warmer: pass complete", "users", len(users))
}
