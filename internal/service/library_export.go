package service

import (
	"context"
	"fmt"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// LibraryExportVersion is the stable schema version embedded in every export.
// Bumped only when the JSON shape changes incompatibly; importers check
// this to decide whether they can read the file.
const LibraryExportVersion = 1

// LibraryExport is the root of a user's library export. Keyed on TMDB IDs
// and natural episode coordinates so the same file round-trips cleanly
// across schema changes.
type LibraryExport struct {
	Version    int           `json:"version"`
	ExportedAt int64         `json:"exported_at"`
	Movies     []MovieExport `json:"movies"`
	Shows      []ShowExport  `json:"shows"`
	Tags       []TagExport   `json:"tags"`
}

type MovieExport struct {
	TMDBID       int                     `json:"tmdb_id"`
	ManualStatus *string                 `json:"manual_status"`
	WatchedAt    *int64                  `json:"watched_at"`
	Notes        string                  `json:"notes,omitempty"`
	Rating       *int                    `json:"rating,omitempty"`
	Tags         []string                `json:"tags,omitempty"`
	WatchEvents  []MovieWatchEventExport `json:"watch_events,omitempty"`
}

type MovieWatchEventExport struct {
	WatchedAt int64  `json:"watched_at"`
	Notes     string `json:"notes,omitempty"`
}

type ShowExport struct {
	TMDBID         int                   `json:"tmdb_id"`
	ManualStatus   *string               `json:"manual_status"`
	WatchedAt      *int64                `json:"watched_at"`
	Notes          string                `json:"notes,omitempty"`
	Rating         *int                  `json:"rating,omitempty"`
	Tags           []string              `json:"tags,omitempty"`
	SeasonRatings  []SeasonRatingExport  `json:"season_ratings,omitempty"`
	EpisodeRatings []EpisodeRatingExport `json:"episode_ratings,omitempty"`
	WatchEvents    []WatchEventExport    `json:"watch_events,omitempty"`
}

type SeasonRatingExport struct {
	SeasonNumber int `json:"season_number"`
	Score        int `json:"score"`
}

type EpisodeRatingExport struct {
	SeasonNumber  int `json:"season_number"`
	EpisodeNumber int `json:"episode_number"`
	Score         int `json:"score"`
}

type WatchEventExport struct {
	SeasonNumber  int    `json:"season_number"`
	EpisodeNumber int    `json:"episode_number"`
	WatchedAt     int64  `json:"watched_at"`
	Notes         string `json:"notes,omitempty"`
}

type TagExport struct {
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
}

// LibraryExporter produces a LibraryExport against the new schema.
type LibraryExporter struct {
	library    model.LibraryRepository
	libraryTag model.LibraryTagRepository
	events     model.WatchEventRepository
	ratings    model.RatingRepositoryV2
	seasons    model.TMDBSeasonRepository
	episodes   model.TMDBEpisodeRepository
	tags       model.TagRepository
}

func NewLibraryExporter(
	library model.LibraryRepository,
	libraryTag model.LibraryTagRepository,
	events model.WatchEventRepository,
	ratings model.RatingRepositoryV2,
	seasons model.TMDBSeasonRepository,
	episodes model.TMDBEpisodeRepository,
	tags model.TagRepository,
) *LibraryExporter {
	return &LibraryExporter{
		library:    library,
		libraryTag: libraryTag,
		events:     events,
		ratings:    ratings,
		seasons:    seasons,
		episodes:   episodes,
		tags:       tags,
	}
}

// Export walks a user's library and returns a fully-populated
// LibraryExport. Pages through the library at the repository's max page
// size — passing a huge PageSize doesn't work because PageRequest.Normalize()
// silently clamps it to model.MaxPageSize.
func (e *LibraryExporter) Export(ctx context.Context, userID string) (*LibraryExport, error) {
	var allItems []model.LibraryView
	var token string
	for {
		page, err := e.library.List(ctx, userID, model.LibraryFilter{}, model.PageRequest{
			PageSize:  model.MaxPageSize,
			PageToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("listing library: %w", err)
		}
		if page == nil {
			break
		}
		page.EnsureItems()
		allItems = append(allItems, page.Items...)
		if page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}
	page := &model.PageResponse[model.LibraryView]{Items: allItems}

	tagMap, err := e.libraryTag.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing library tags: %w", err)
	}

	allTags, err := e.tags.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}

	out := &LibraryExport{
		Version:    LibraryExportVersion,
		ExportedAt: time.Now().UTC().Unix(),
		Movies:     []MovieExport{},
		Shows:      []ShowExport{},
		Tags:       make([]TagExport, 0, len(allTags)),
	}
	for _, t := range allTags {
		out.Tags = append(out.Tags, TagExport{
			Name:      t.Name,
			CreatedAt: t.CreatedAt.Unix(),
		})
	}

	for _, v := range page.Items {
		tagNames := make([]string, 0, len(tagMap[v.Entry.ID]))
		for _, t := range tagMap[v.Entry.ID] {
			tagNames = append(tagNames, t.Name)
		}

		var manualStatus *string
		if v.Entry.ManualStatus != nil {
			ms := string(*v.Entry.ManualStatus)
			manualStatus = &ms
		}

		switch v.Entry.MediaType {
		case model.MediaTypeMovie:
			if v.Movie == nil {
				continue
			}
			me := MovieExport{
				TMDBID:       v.Movie.TMDBID,
				ManualStatus: manualStatus,
				WatchedAt:    v.Entry.WatchedAt,
				Notes:        v.Entry.Notes,
				Tags:         tagNames,
			}
			if r, err := e.ratings.GetMovie(ctx, userID, v.Movie.ID); err == nil && r != nil {
				s := r.Score
				me.Rating = &s
			}
			events, err := e.events.ListForMovie(ctx, userID, v.Movie.ID)
			if err == nil {
				for _, ev := range events {
					me.WatchEvents = append(me.WatchEvents, MovieWatchEventExport{
						WatchedAt: ev.WatchedAt,
						Notes:     ev.Notes,
					})
				}
			}
			out.Movies = append(out.Movies, me)

		case model.MediaTypeTV:
			if v.Show == nil {
				continue
			}
			se := ShowExport{
				TMDBID:       v.Show.TMDBID,
				ManualStatus: manualStatus,
				WatchedAt:    v.Entry.WatchedAt,
				Notes:        v.Entry.Notes,
				Tags:         tagNames,
			}
			if r, err := e.ratings.GetShow(ctx, userID, v.Show.ID); err == nil && r != nil {
				s := r.Score
				se.Rating = &s
			}

			seasons, err := e.seasons.ListByShow(ctx, v.Show.ID)
			if err != nil {
				return nil, fmt.Errorf("listing seasons for %s: %w", v.Show.Title, err)
			}
			seasonNumByID := make(map[string]int, len(seasons))
			for _, s := range seasons {
				seasonNumByID[s.ID] = s.SeasonNumber
			}
			seasonRatingMap, _ := e.ratings.ListSeasonRatingsByShow(ctx, userID, v.Show.ID)
			for sid, score := range seasonRatingMap {
				if sn, ok := seasonNumByID[sid]; ok {
					se.SeasonRatings = append(se.SeasonRatings, SeasonRatingExport{
						SeasonNumber: sn,
						Score:        score,
					})
				}
			}

			episodes, err := e.episodes.ListByShow(ctx, v.Show.ID)
			if err != nil {
				return nil, fmt.Errorf("listing episodes for %s: %w", v.Show.Title, err)
			}
			epRatingMap, _ := e.ratings.ListEpisodeRatingsByShow(ctx, userID, v.Show.ID)
			for _, ep := range episodes {
				if score, ok := epRatingMap[ep.ID]; ok {
					se.EpisodeRatings = append(se.EpisodeRatings, EpisodeRatingExport{
						SeasonNumber:  seasonNumByID[ep.SeasonID],
						EpisodeNumber: ep.EpisodeNumber,
						Score:         score,
					})
				}
				events, err := e.events.ListForEpisode(ctx, userID, ep.ID)
				if err != nil {
					continue
				}
				for _, ev := range events {
					se.WatchEvents = append(se.WatchEvents, WatchEventExport{
						SeasonNumber:  seasonNumByID[ep.SeasonID],
						EpisodeNumber: ep.EpisodeNumber,
						WatchedAt:     ev.WatchedAt,
						Notes:         ev.Notes,
					})
				}
			}

			out.Shows = append(out.Shows, se)
		}
	}

	return out, nil
}
