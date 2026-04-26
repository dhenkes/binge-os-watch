package jobs

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/dhenkes/binge-os-watch/internal/tmdb"
)

// KeywordScanner runs saved keyword searches against TMDB on a regular interval.
type KeywordScanner struct {
	watches  model.KeywordWatchRepository
	library  model.LibraryRepository
	tmdb     *tmdb.Client
	interval time.Duration
}

func NewKeywordScanner(
	watches model.KeywordWatchRepository,
	library model.LibraryRepository,
	tmdbClient *tmdb.Client,
	interval time.Duration,
) *KeywordScanner {
	return &KeywordScanner{
		watches:  watches,
		library:  library,
		tmdb:     tmdbClient,
		interval: interval,
	}
}

// Run starts the keyword scanner loop. It blocks until ctx is cancelled.
func (j *KeywordScanner) Run(ctx context.Context) {
	slog.Info("keyword scanner started", "interval", j.interval)

	// Run immediately on startup, then on interval.
	j.scan(ctx)

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			j.scan(ctx)
		case <-ctx.Done():
			slog.Info("keyword scanner stopped")
			return
		}
	}
}

func (j *KeywordScanner) scan(ctx context.Context) {
	slog.Info("keyword scanner running")

	// Get all keyword watches across all users.
	// We need to list per-user. Get all users who have watches by querying directly.
	rows, err := j.watches.ListAll(ctx)
	if err != nil {
		slog.Error("keyword scanner: listing watches", "error", err)
		return
	}

	for _, kw := range rows {
		j.scanKeyword(ctx, &kw)
	}

	slog.Info("keyword scanner complete")
}

func (j *KeywordScanner) scanKeyword(ctx context.Context, kw *model.KeywordWatch) {
	types := strings.Split(kw.MediaTypes, ",")

	for _, mt := range types {
		mt = strings.TrimSpace(mt)
		if mt != "movie" && mt != "tv" {
			continue
		}

		var results []tmdb.SearchResult
		switch mt {
		case "movie":
			resp, err := j.tmdb.SearchMovies(ctx, kw.Keyword, 1, 0)
			if err != nil {
				slog.Warn("keyword scanner: search failed", "keyword", kw.Keyword, "type", mt, "error", err)
				continue
			}
			results = resp.Results
		case "tv":
			resp, err := j.tmdb.SearchTV(ctx, kw.Keyword, 1, 0)
			if err != nil {
				slog.Warn("keyword scanner: search failed", "keyword", kw.Keyword, "type", mt, "error", err)
				continue
			}
			results = resp.Results
		}

		for _, r := range results {
			mediaType := model.MediaType(mt)

			// Skip if already in user's library.
			existing, _ := j.library.GetByTMDBID(ctx, kw.UserID, r.ID, mediaType)
			if existing != nil {
				continue
			}

			// Skip if result already exists for this watch.
			exists, _ := j.watches.ResultExists(ctx, kw.ID, r.ID, mediaType)
			if exists {
				continue
			}

			// Insert new pending result.
			result := &model.KeywordResult{
				KeywordWatchID: kw.ID,
				TMDBID:         r.ID,
				MediaType:      mediaType,
				Title:          r.DisplayTitle(),
				PosterPath:     r.PosterPath,
				ReleaseDate:    r.DisplayDate(),
				Status:         model.KeywordResultPending,
			}
			if err := j.watches.CreateResult(ctx, result); err != nil {
				slog.Warn("keyword scanner: creating result", "keyword", kw.Keyword, "tmdb_id", r.ID, "error", err)
			}
		}
	}
}
