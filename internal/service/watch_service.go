package service

import (
	"context"
	"fmt"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// WatchServiceImpl owns "mark watched" business logic for both episodes
// and movies in the new schema. Every mutator refreshes the denormalized
// user_library.watched_at after inserting or deleting a watch_event so
// list queries stay cheap.
type WatchServiceImpl struct {
	events      model.WatchEventRepository
	library     model.LibraryRepository
	seasons     model.TMDBSeasonRepository
	episodes    model.TMDBEpisodeRepository
	webhooks    WebhookDispatcher
	now         func() int64
}

var _ model.WatchService = (*WatchServiceImpl)(nil)

func NewWatchService(
	events model.WatchEventRepository,
	library model.LibraryRepository,
	seasons model.TMDBSeasonRepository,
	episodes model.TMDBEpisodeRepository,
) *WatchServiceImpl {
	return &WatchServiceImpl{
		events:   events,
		library:  library,
		seasons:  seasons,
		episodes: episodes,
		now:      func() int64 { return time.Now().UTC().Unix() },
	}
}

func (s *WatchServiceImpl) SetWebhookDispatcher(d WebhookDispatcher) {
	s.webhooks = d
}

// WatchEpisode inserts a watch_event for an episode and refreshes the
// parent show's denormalized watched_at and derived status.
func (s *WatchServiceImpl) WatchEpisode(ctx context.Context, userID, episodeID string, watchedAt int64, notes string) error {
	ep, err := s.episodes.GetByID(ctx, episodeID)
	if err != nil {
		return err
	}
	if watchedAt == 0 {
		watchedAt = s.now()
	}
	if err := s.events.Create(ctx, &model.WatchEvent{
		UserID:    userID,
		EpisodeID: &ep.ID,
		WatchedAt: watchedAt,
		Notes:     notes,
	}); err != nil {
		return err
	}

	season, err := s.seasons.GetByID(ctx, ep.SeasonID)
	if err != nil {
		return err
	}
	view, err := s.library.GetByShow(ctx, userID, season.ShowID)
	if err != nil {
		return err
	}
	return s.refreshLibraryRow(ctx, view, watchedAt)
}

// UnwatchEpisode walks a rewatch back one click by deleting only the
// most-recent watch_event row for the episode.
func (s *WatchServiceImpl) UnwatchEpisode(ctx context.Context, userID, episodeID string) error {
	if err := s.events.DeleteLatestForEpisode(ctx, userID, episodeID); err != nil {
		return err
	}
	return s.refreshShowByEpisode(ctx, userID, episodeID)
}

// UnwatchAllForEpisode wipes every watch row for an episode. Used by the
// "mark unwatched" affordance on the episode detail page.
func (s *WatchServiceImpl) UnwatchAllForEpisode(ctx context.Context, userID, episodeID string) error {
	if err := s.events.DeleteAllForEpisode(ctx, userID, episodeID); err != nil {
		return err
	}
	return s.refreshShowByEpisode(ctx, userID, episodeID)
}

func (s *WatchServiceImpl) WatchMovie(ctx context.Context, userID, movieID string, watchedAt int64, notes string) error {
	if watchedAt == 0 {
		watchedAt = s.now()
	}
	if err := s.events.Create(ctx, &model.WatchEvent{
		UserID:    userID,
		MovieID:   &movieID,
		WatchedAt: watchedAt,
		Notes:     notes,
	}); err != nil {
		return err
	}
	view, err := s.library.GetByMovie(ctx, userID, movieID)
	if err != nil {
		return err
	}
	return s.refreshLibraryRow(ctx, view, watchedAt)
}

func (s *WatchServiceImpl) UnwatchMovie(ctx context.Context, userID, movieID string) error {
	if err := s.events.DeleteLatestForMovie(ctx, userID, movieID); err != nil {
		return err
	}
	view, err := s.library.GetByMovie(ctx, userID, movieID)
	if err != nil {
		return err
	}
	// Fall back to the previous remaining event's watched_at, if any.
	latest, _ := s.events.LatestForMovie(ctx, userID, movieID)
	var newWatchedAt *int64
	if latest != nil {
		v := latest.WatchedAt
		newWatchedAt = &v
	}
	if err := s.library.UpdateWatchedAt(ctx, view.Entry.ID, newWatchedAt); err != nil {
		return err
	}
	return s.recalcDerived(ctx, view)
}

// WatchNext marks the next unwatched aired regular episode watched and
// returns it, or nil if the show is already caught up.
func (s *WatchServiceImpl) WatchNext(ctx context.Context, userID, showID string) (*model.TMDBEpisode, error) {
	next, err := s.events.NextUnwatched(ctx, userID, showID)
	if err != nil {
		return nil, err
	}
	if err := s.WatchEpisode(ctx, userID, next.ID, s.now(), ""); err != nil {
		return nil, err
	}
	return next, nil
}

// WatchAllInSeason marks every episode in a season watched with the
// current timestamp. Rewatches behave as expected.
func (s *WatchServiceImpl) WatchAllInSeason(ctx context.Context, userID, seasonID string) error {
	eps, err := s.episodes.ListBySeason(ctx, seasonID)
	if err != nil {
		return err
	}
	now := s.now()
	for _, ep := range eps {
		if err := s.events.Create(ctx, &model.WatchEvent{
			UserID:    userID,
			EpisodeID: &ep.ID,
			WatchedAt: now,
		}); err != nil {
			return err
		}
	}
	season, err := s.seasons.GetByID(ctx, seasonID)
	if err != nil {
		return err
	}
	view, err := s.library.GetByShow(ctx, userID, season.ShowID)
	if err != nil {
		return err
	}
	return s.refreshLibraryRow(ctx, view, now)
}

// WatchUpToEpisode marks every episode in the show whose air_date is on
// or before the target episode's air_date as watched. Includes specials
// that aired earlier. Uses the current time as each event's watched_at.
func (s *WatchServiceImpl) WatchUpToEpisode(ctx context.Context, userID, episodeID string) error {
	return s.WatchUpToEpisodeWithDate(ctx, userID, episodeID, "today", 0)
}

// WatchUpToEpisodeWithDate is the date-mode-aware variant. mode is one of
// "today", "release", or "custom":
//   - "today": every inserted event uses now
//   - "release": each event uses the episode's own air_date (falls back to
//     now for episodes without an air_date, which in practice shouldn't
//     happen here because we already filter on air_date)
//   - "custom": every event uses customAt (unix seconds)
func (s *WatchServiceImpl) WatchUpToEpisodeWithDate(ctx context.Context, userID, episodeID, mode string, customAt int64) error {
	target, err := s.episodes.GetByID(ctx, episodeID)
	if err != nil {
		return err
	}
	season, err := s.seasons.GetByID(ctx, target.SeasonID)
	if err != nil {
		return err
	}
	allEps, err := s.episodes.ListByShow(ctx, season.ShowID)
	if err != nil {
		return err
	}

	now := s.now()
	var cutoff int64
	if target.AirDate != nil {
		cutoff = *target.AirDate
	}
	// Seen map so we don't double-insert rewatches for the bulk op.
	seen, err := s.events.WatchedMapForShow(ctx, userID, season.ShowID)
	if err != nil {
		return err
	}

	watchedAtFor := func(ep *model.TMDBEpisode) int64 {
		switch mode {
		case "release":
			if ep.AirDate != nil && *ep.AirDate > 0 {
				return *ep.AirDate
			}
			return now
		case "custom":
			if customAt > 0 {
				return customAt
			}
			return now
		default:
			return now
		}
	}

	// Track the most recent watched_at we insert so the library row's
	// denormalized watched_at reflects reality after the bulk op.
	latestInserted := int64(0)

	for i := range allEps {
		ep := &allEps[i]
		if ep.AirDate == nil {
			continue
		}
		if cutoff != 0 && *ep.AirDate > cutoff {
			continue
		}
		if _, already := seen[ep.ID]; already {
			continue
		}
		at := watchedAtFor(ep)
		if err := s.events.Create(ctx, &model.WatchEvent{
			UserID:    userID,
			EpisodeID: &ep.ID,
			WatchedAt: at,
		}); err != nil {
			return err
		}
		if at > latestInserted {
			latestInserted = at
		}
	}
	// Always stamp the target itself.
	if _, already := seen[target.ID]; !already {
		at := watchedAtFor(target)
		if err := s.events.Create(ctx, &model.WatchEvent{
			UserID:    userID,
			EpisodeID: &target.ID,
			WatchedAt: at,
		}); err != nil {
			return err
		}
		if at > latestInserted {
			latestInserted = at
		}
	}
	view, err := s.library.GetByShow(ctx, userID, season.ShowID)
	if err != nil {
		return err
	}
	refreshAt := latestInserted
	if refreshAt == 0 {
		refreshAt = now
	}
	return s.refreshLibraryRow(ctx, view, refreshAt)
}

// UnwatchUpToEpisode removes every watch row (not just the latest) for
// episodes whose air_date is on or before the target. Mirrors
// WatchUpToEpisode.
func (s *WatchServiceImpl) UnwatchUpToEpisode(ctx context.Context, userID, episodeID string) error {
	target, err := s.episodes.GetByID(ctx, episodeID)
	if err != nil {
		return err
	}
	season, err := s.seasons.GetByID(ctx, target.SeasonID)
	if err != nil {
		return err
	}
	allEps, err := s.episodes.ListByShow(ctx, season.ShowID)
	if err != nil {
		return err
	}
	var cutoff int64
	if target.AirDate != nil {
		cutoff = *target.AirDate
	}
	for _, ep := range allEps {
		if ep.AirDate == nil {
			continue
		}
		if cutoff != 0 && *ep.AirDate > cutoff {
			continue
		}
		if err := s.events.DeleteAllForEpisode(ctx, userID, ep.ID); err != nil {
			return err
		}
	}
	view, err := s.library.GetByShow(ctx, userID, season.ShowID)
	if err != nil {
		return err
	}
	return s.refreshLibraryRow(ctx, view, 0)
}

// RecalcStatus refreshes the derived status for a library entry. Used by
// metadata_sync so newly-aired episodes flip a previously-watched show
// back to "watching" on the next tick.
func (s *WatchServiceImpl) RecalcStatus(ctx context.Context, libraryID string) error {
	view, err := s.library.GetByID(ctx, libraryID)
	if err != nil {
		return err
	}
	return s.recalcDerived(ctx, view)
}

// refreshShowByEpisode is a helper that finds the library row for the
// show containing an episode and runs the denormalized refresh.
func (s *WatchServiceImpl) refreshShowByEpisode(ctx context.Context, userID, episodeID string) error {
	ep, err := s.episodes.GetByID(ctx, episodeID)
	if err != nil {
		return err
	}
	season, err := s.seasons.GetByID(ctx, ep.SeasonID)
	if err != nil {
		return err
	}
	view, err := s.library.GetByShow(ctx, userID, season.ShowID)
	if err != nil {
		return err
	}
	latest, _ := s.events.LatestForEpisode(ctx, userID, episodeID)
	var newAt *int64
	if latest != nil {
		v := latest.WatchedAt
		newAt = &v
	}
	if err := s.library.UpdateWatchedAt(ctx, view.Entry.ID, newAt); err != nil {
		return err
	}
	return s.recalcDerived(ctx, view)
}

// refreshLibraryRow bumps the denormalized watched_at to the given value
// (if non-zero) and re-derives the status.
func (s *WatchServiceImpl) refreshLibraryRow(ctx context.Context, view *model.LibraryView, watchedAt int64) error {
	if watchedAt > 0 {
		if err := s.library.UpdateWatchedAt(ctx, view.Entry.ID, &watchedAt); err != nil {
			return err
		}
	}
	return s.recalcDerived(ctx, view)
}

// recalcDerived computes the auto-derived status for the given library
// entry and applies it if the user hasn't set a manual override to a
// terminal state (on_hold / dropped remain unless the user explicitly
// unblocks them by toggling it off via the UI).
func (s *WatchServiceImpl) recalcDerived(ctx context.Context, view *model.LibraryView) error {
	// Movies: derived status depends on whether any watch event exists.
	if view.Entry.MediaType == model.MediaTypeMovie {
		// Movies don't auto-derive across watched/watching — only via
		// the manual status on the detail page. No-op here to avoid
		// clobbering the user's choice.
		return nil
	}
	if view.Entry.ShowID == nil {
		return fmt.Errorf("tv library entry with no show id: %s", view.Entry.ID)
	}
	watched, total, err := s.events.ProgressForShow(ctx, view.Entry.UserID, *view.Entry.ShowID)
	if err != nil {
		return err
	}
	derived := model.DeriveStatus(total, watched, false)

	// If the user has an explicit manual status, only clear it when we
	// can confidently auto-derive a new one (i.e. they just watched or
	// unwatched an episode). We know that's the case here.
	if view.Entry.ManualStatus != nil && *view.Entry.ManualStatus == derived {
		return nil
	}
	return s.library.SetManualStatus(ctx, view.Entry.ID, &derived)
}
