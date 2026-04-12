package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// LibraryImporter reads a LibraryExport JSON and applies it to a user's
// library. Idempotent per-item: missing shows/movies are added through
// LibraryService (which fetches + upserts the catalog), already-present
// items are skipped, and progress/ratings are layered on top of either
// case.
type LibraryImporter struct {
	library     model.LibraryService
	libraryRepo model.LibraryRepository
	watch       model.WatchService
	ratings     model.RatingServiceV2
	tags        model.TagService
	libraryTag  model.LibraryTagRepository
	tagRepo     model.TagRepository
	seasonRepo  model.TMDBSeasonRepository
	episodeRepo model.TMDBEpisodeRepository
}

func NewLibraryImporter(
	library model.LibraryService,
	libraryRepo model.LibraryRepository,
	watch model.WatchService,
	ratings model.RatingServiceV2,
	tags model.TagService,
	libraryTag model.LibraryTagRepository,
	tagRepo model.TagRepository,
	seasonRepo model.TMDBSeasonRepository,
	episodeRepo model.TMDBEpisodeRepository,
) *LibraryImporter {
	return &LibraryImporter{
		library:     library,
		libraryRepo: libraryRepo,
		watch:       watch,
		ratings:     ratings,
		tags:        tags,
		libraryTag:  libraryTag,
		tagRepo:     tagRepo,
		seasonRepo:  seasonRepo,
		episodeRepo: episodeRepo,
	}
}

type ImportResult struct {
	MoviesAdded   int
	MoviesSkipped int
	ShowsAdded    int
	ShowsSkipped  int
	Errors        []string
}

func (i *LibraryImporter) Import(ctx context.Context, userID string, data *LibraryExport) (*ImportResult, error) {
	if data == nil {
		return nil, fmt.Errorf("nil export")
	}
	if data.Version != LibraryExportVersion {
		return nil, fmt.Errorf("unsupported export version %d (expected %d)", data.Version, LibraryExportVersion)
	}

	res := &ImportResult{}

	tagByName := map[string]string{}
	existing, err := i.tagRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing existing tags: %w", err)
	}
	for _, t := range existing {
		tagByName[t.Name] = t.ID
	}
	for _, t := range data.Tags {
		if _, ok := tagByName[t.Name]; ok {
			continue
		}
		created, err := i.tags.Create(ctx, userID, t.Name)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("tag %q: %v", t.Name, err))
			continue
		}
		tagByName[t.Name] = created.ID
	}

	for _, me := range data.Movies {
		existed, err := i.importMovie(ctx, userID, me, tagByName)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("movie tmdb:%d: %v", me.TMDBID, err))
			continue
		}
		if existed {
			res.MoviesSkipped++
		} else {
			res.MoviesAdded++
		}
	}

	for _, se := range data.Shows {
		existed, err := i.importShow(ctx, userID, se, tagByName)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("show tmdb:%d: %v", se.TMDBID, err))
			continue
		}
		if existed {
			res.ShowsSkipped++
		} else {
			res.ShowsAdded++
		}
	}

	return res, nil
}

func (i *LibraryImporter) importMovie(ctx context.Context, userID string, me MovieExport, tagByName map[string]string) (existed bool, err error) {
	view, existed, err := i.resolveOrAdd(ctx, userID, me.TMDBID, model.MediaTypeMovie)
	if err != nil {
		return false, err
	}

	if me.ManualStatus != nil {
		ms := model.MediaStatus(*me.ManualStatus)
		if err := i.library.SetStatus(ctx, view.Entry.ID, &ms); err != nil {
			slog.Warn("import: set movie status", "tmdb_id", me.TMDBID, "error", err)
		}
	}
	if me.WatchedAt != nil {
		v := *me.WatchedAt
		if err := i.library.UpdateWatchedAt(ctx, view.Entry.ID, &v); err != nil {
			slog.Warn("import: movie watched_at", "tmdb_id", me.TMDBID, "error", err)
		}
	}
	if me.Notes != "" {
		if err := i.library.UpdateNotes(ctx, view.Entry.ID, me.Notes); err != nil {
			slog.Warn("import: movie notes", "tmdb_id", me.TMDBID, "error", err)
		}
	}
	if me.Rating != nil && view.Movie != nil {
		if err := i.ratings.RateMovie(ctx, userID, view.Movie.ID, *me.Rating); err != nil {
			slog.Warn("import: movie rating", "tmdb_id", me.TMDBID, "error", err)
		}
	}
	if view.Movie != nil {
		for _, ev := range me.WatchEvents {
			if err := i.watch.WatchMovie(ctx, userID, view.Movie.ID, ev.WatchedAt, ev.Notes); err != nil {
				slog.Warn("import: movie watch event", "tmdb_id", me.TMDBID, "error", err)
			}
		}
	}
	i.applyTags(ctx, view.Entry.ID, me.Tags, tagByName)
	return existed, nil
}

func (i *LibraryImporter) importShow(ctx context.Context, userID string, se ShowExport, tagByName map[string]string) (existed bool, err error) {
	view, existed, err := i.resolveOrAdd(ctx, userID, se.TMDBID, model.MediaTypeTV)
	if err != nil {
		return false, err
	}

	if se.ManualStatus != nil {
		ms := model.MediaStatus(*se.ManualStatus)
		if err := i.library.SetStatus(ctx, view.Entry.ID, &ms); err != nil {
			slog.Warn("import: set show status", "tmdb_id", se.TMDBID, "error", err)
		}
	}
	if se.Notes != "" {
		if err := i.library.UpdateNotes(ctx, view.Entry.ID, se.Notes); err != nil {
			slog.Warn("import: show notes", "tmdb_id", se.TMDBID, "error", err)
		}
	}
	if se.Rating != nil && view.Show != nil {
		if err := i.ratings.RateShow(ctx, userID, view.Show.ID, *se.Rating); err != nil {
			slog.Warn("import: show rating", "tmdb_id", se.TMDBID, "error", err)
		}
	}
	i.applyTags(ctx, view.Entry.ID, se.Tags, tagByName)

	if view.Show == nil {
		return existed, nil
	}

	seasons, err := i.seasonRepo.ListByShow(ctx, view.Show.ID)
	if err != nil {
		return existed, fmt.Errorf("listing seasons: %w", err)
	}
	seasonIDByNum := make(map[int]string, len(seasons))
	seasonNumByID := make(map[string]int, len(seasons))
	for _, s := range seasons {
		seasonIDByNum[s.SeasonNumber] = s.ID
		seasonNumByID[s.ID] = s.SeasonNumber
	}
	for _, sr := range se.SeasonRatings {
		id, ok := seasonIDByNum[sr.SeasonNumber]
		if !ok {
			continue
		}
		if err := i.ratings.RateSeason(ctx, userID, id, sr.Score); err != nil {
			slog.Warn("import: season rating", "tmdb_id", se.TMDBID, "season", sr.SeasonNumber, "error", err)
		}
	}

	episodes, err := i.episodeRepo.ListByShow(ctx, view.Show.ID)
	if err != nil {
		return existed, fmt.Errorf("listing episodes: %w", err)
	}
	epIDByNatKey := make(map[[2]int]string, len(episodes))
	for _, ep := range episodes {
		sn := seasonNumByID[ep.SeasonID]
		epIDByNatKey[[2]int{sn, ep.EpisodeNumber}] = ep.ID
	}

	for _, er := range se.EpisodeRatings {
		id, ok := epIDByNatKey[[2]int{er.SeasonNumber, er.EpisodeNumber}]
		if !ok {
			continue
		}
		if err := i.ratings.RateEpisode(ctx, userID, id, er.Score); err != nil {
			slog.Warn("import: episode rating", "tmdb_id", se.TMDBID, "s", er.SeasonNumber, "e", er.EpisodeNumber, "error", err)
		}
	}

	for _, we := range se.WatchEvents {
		id, ok := epIDByNatKey[[2]int{we.SeasonNumber, we.EpisodeNumber}]
		if !ok {
			continue
		}
		if err := i.watch.WatchEpisode(ctx, userID, id, we.WatchedAt, we.Notes); err != nil {
			slog.Warn("import: watch event", "tmdb_id", se.TMDBID, "s", we.SeasonNumber, "e", we.EpisodeNumber, "error", err)
		}
	}

	if se.WatchedAt != nil {
		v := *se.WatchedAt
		if err := i.library.UpdateWatchedAt(ctx, view.Entry.ID, &v); err != nil {
			slog.Warn("import: show watched_at", "tmdb_id", se.TMDBID, "error", err)
		}
	}
	return existed, nil
}

func (i *LibraryImporter) resolveOrAdd(ctx context.Context, userID string, tmdbID int, mt model.MediaType) (*model.LibraryView, bool, error) {
	existing, err := i.libraryRepo.GetByTMDBID(ctx, userID, tmdbID, mt)
	if err == nil && existing != nil {
		return existing, true, nil
	}
	added, err := i.library.Add(ctx, userID, tmdbID, mt)
	if err != nil {
		return nil, false, err
	}
	return added, false, nil
}

func (i *LibraryImporter) applyTags(ctx context.Context, libraryID string, names []string, tagByName map[string]string) {
	for _, name := range names {
		id, ok := tagByName[name]
		if !ok {
			continue
		}
		if err := i.libraryTag.Add(ctx, libraryID, id); err != nil {
			slog.Warn("import: add tag", "library_id", libraryID, "tag", name, "error", err)
		}
	}
}
