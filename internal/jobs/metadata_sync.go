package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// MetadataSync refreshes the shared TMDB catalog on a regular interval.
// Under the new schema the catalog is per-installation (not per-user),
// so one pass refreshes everyone's shows at once. LibraryService drives
// the actual work — this job is just a ticker around RefreshAll.
type MetadataSync struct {
	library  model.LibraryService
	interval time.Duration
}

func NewMetadataSync(library model.LibraryService, interval time.Duration) *MetadataSync {
	return &MetadataSync{library: library, interval: interval}
}

// Run starts the metadata sync loop. It blocks until ctx is cancelled.
func (j *MetadataSync) Run(ctx context.Context) {
	slog.Info("metadata sync started", "interval", j.interval)

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			j.sync(ctx)
		case <-ctx.Done():
			slog.Info("metadata sync stopped")
			return
		}
	}
}

func (j *MetadataSync) sync(ctx context.Context) {
	slog.Info("metadata sync running")
	if err := j.library.RefreshAll(ctx); err != nil {
		slog.Warn("metadata sync: refresh all", "error", err)
	}
	slog.Info("metadata sync complete")
}
