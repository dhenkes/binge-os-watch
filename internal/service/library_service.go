package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/dhenkes/binge-os-watch/internal/tmdb"
)

// LibraryServiceImpl owns the business logic for adding/removing library
// entries in the new schema. Add inserts a placeholder catalog row and
// the per-user user_library row in one fast transaction, then hands the
// heavy lifting off to the TMDBJobRunner — the UI never blocks on a
// TMDB fetch. RefreshCatalog follows the same split.
type LibraryServiceImpl struct {
	txFunc   model.TxFunc
	shows    model.TMDBShowRepository
	movies   model.TMDBMovieRepository
	seasons  model.TMDBSeasonRepository
	episodes model.TMDBEpisodeRepository
	library  model.LibraryRepository
	tmdb     *tmdb.Client
	webhooks WebhookDispatcher
	// tmdbJobs enqueues background TMDB fetches. Assigned post-construction
	// via SetTMDBJobRunner to break the cycle (the runner holds a pointer
	// back to this service for CompleteCatalogFetch).
	tmdbJobs CatalogJobEnqueuer
	now      func() int64
}

// CatalogJobEnqueuer is the tiny surface area LibraryServiceImpl needs
// from the TMDB job runner. Defined as an interface so we can swap it
// out in tests.
type CatalogJobEnqueuer interface {
	EnqueueAddCatalog(ctx context.Context, userID string, tmdbID int, mediaType string) (string, error)
	EnqueueRefreshCatalog(ctx context.Context, tmdbID int, mediaType string) (string, error)
}

var _ model.LibraryService = (*LibraryServiceImpl)(nil)

func NewLibraryService(
	txFunc model.TxFunc,
	shows model.TMDBShowRepository,
	movies model.TMDBMovieRepository,
	seasons model.TMDBSeasonRepository,
	episodes model.TMDBEpisodeRepository,
	library model.LibraryRepository,
	tmdbClient *tmdb.Client,
) *LibraryServiceImpl {
	return &LibraryServiceImpl{
		txFunc:   txFunc,
		shows:    shows,
		movies:   movies,
		seasons:  seasons,
		episodes: episodes,
		library:  library,
		tmdb:     tmdbClient,
		now:      func() int64 { return time.Now().UTC().Unix() },
	}
}

// SetWebhookDispatcher wires the webhook dispatcher after construction
// (avoids a circular init between library/webhook services).
func (s *LibraryServiceImpl) SetWebhookDispatcher(d WebhookDispatcher) {
	s.webhooks = d
}

// SetTMDBJobRunner wires the background TMDB job runner after
// construction. See CatalogJobEnqueuer for why this is a setter.
func (s *LibraryServiceImpl) SetTMDBJobRunner(r CatalogJobEnqueuer) {
	s.tmdbJobs = r
}

// Add is the no-stub convenience wrapper. Callers that have search
// results should use AddWithStub to avoid a "Loading…" placeholder.
func (s *LibraryServiceImpl) Add(ctx context.Context, userID string, tmdbID int, mediaType model.MediaType) (*model.LibraryView, error) {
	return s.AddWithStub(ctx, userID, tmdbID, mediaType, nil)
}

// AddWithStub is the async add path: inserts a minimal catalog row and
// the user_library entry synchronously, then enqueues a background TMDB
// job to flesh the catalog out. Returns as soon as the DB writes finish
// — typically a few ms — so the UI can redirect without blocking.
//
// The catalog row is keyed by tmdb_id (UNIQUE index), so subsequent Adds
// of the same item reuse the row and a second fetch job will upsert the
// same columns idempotently. RefreshedAt=0 is the "placeholder" marker
// the UI/metadata-sync can use to tell "never fetched" from "stale".
func (s *LibraryServiceImpl) AddWithStub(
	ctx context.Context,
	userID string,
	tmdbID int,
	mediaType model.MediaType,
	stub *model.AddStub,
) (*model.LibraryView, error) {
	if existing, err := s.library.GetByTMDBID(ctx, userID, tmdbID, mediaType); err == nil && existing != nil {
		return existing, model.NewAlreadyExists("item already in library")
	}
	if mediaType != model.MediaTypeMovie && mediaType != model.MediaTypeTV {
		return nil, model.NewInvalidArgument("media_type must be 'movie' or 'tv'")
	}

	title := "Loading…"
	overview, posterPath, releaseRaw := "", "", ""
	if stub != nil {
		if stub.Title != "" {
			title = stub.Title
		}
		overview = stub.Overview
		posterPath = stub.PosterPath
		releaseRaw = stub.ReleaseDate
	}

	var view *model.LibraryView
	err := s.txFunc(ctx, func(ctx context.Context) error {
		entry := &model.LibraryEntry{
			UserID:    userID,
			MediaType: mediaType,
			CreatedAt: s.now(),
			UpdatedAt: s.now(),
		}
		switch mediaType {
		case model.MediaTypeMovie:
			movie := &model.TMDBMovie{
				TMDBID:      tmdbID,
				Title:       title,
				Overview:    overview,
				PosterPath:  posterPath,
				ReleaseDate: parseTMDBDate(releaseRaw),
				RefreshedAt: 0, // placeholder marker
			}
			if err := s.movies.Upsert(ctx, movie); err != nil {
				return err
			}
			entry.MovieID = &movie.ID
		case model.MediaTypeTV:
			show := &model.TMDBShow{
				TMDBID:       tmdbID,
				Title:        title,
				Overview:     overview,
				PosterPath:   posterPath,
				FirstAirDate: parseTMDBDate(releaseRaw),
				RefreshedAt:  0, // placeholder marker
			}
			if err := s.shows.Upsert(ctx, show); err != nil {
				return err
			}
			entry.ShowID = &show.ID
		}
		if err := s.library.Create(ctx, entry); err != nil {
			return err
		}
		v, err := s.library.GetByID(ctx, entry.ID)
		if err != nil {
			return err
		}
		view = v
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Fire-and-forget background catalog fetch.
	if s.tmdbJobs != nil {
		if _, err := s.tmdbJobs.EnqueueAddCatalog(ctx, userID, tmdbID, string(mediaType)); err != nil {
			// Non-fatal: the library entry is already live, the user
			// will just see the placeholder title until the next
			// metadata-sync tick picks it up.
			_ = err
		}
	}

	if s.webhooks != nil && view != nil {
		s.webhooks.Dispatch(ctx, userID, model.WebhookEventAdded, map[string]any{
			"media_id":   view.Entry.ID,
			"title":      title,
			"media_type": string(mediaType),
			"status":     string(model.MediaStatusPlanToWatch),
		})
	}
	return view, nil
}

// CompleteCatalogFetch is the worker-side counterpart to AddWithStub. It
// does all the TMDB HTTP calls outside of any DB transaction, then upserts
// the resulting rows. Idempotent — safe to re-run against a partially
// populated catalog.
func (s *LibraryServiceImpl) CompleteCatalogFetch(ctx context.Context, tmdbID int, mediaType string) error {
	switch model.MediaType(mediaType) {
	case model.MediaTypeMovie:
		details, err := s.tmdb.GetMovie(ctx, tmdbID)
		if err != nil {
			return fmt.Errorf("fetching movie from TMDB: %w", err)
		}
		movie := &model.TMDBMovie{
			TMDBID:         details.ID,
			Title:          details.Title,
			Overview:       details.Overview,
			PosterPath:     details.PosterPath,
			BackdropPath:   details.BackdropPath,
			ReleaseDate:    parseTMDBDate(details.ReleaseDate),
			RuntimeMinutes: details.Runtime,
			Genres:         genreNames(details.Genres),
			TMDBStatus:     details.Status,
			RefreshedAt:    s.now(),
		}
		return s.txFunc(ctx, func(ctx context.Context) error {
			return s.movies.Upsert(ctx, movie)
		})
	case model.MediaTypeTV:
		pre, err := s.fetchShowCatalog(ctx, tmdbID)
		if err != nil {
			return err
		}
		return s.txFunc(ctx, func(ctx context.Context) error {
			_, err := s.persistShowCatalog(ctx, pre)
			return err
		})
	default:
		return fmt.Errorf("unknown media type: %q", mediaType)
	}
}

// preloadedShow is the result of fetchShowCatalog: all TMDB HTTP
// responses needed to upsert a show's full catalog, stashed in memory so
// the DB transaction only has to do writes.
type preloadedShow struct {
	show    *tmdb.TVDetails
	seasons map[int]*tmdb.SeasonDetails // keyed by season number
}

// fetchShowCatalog pulls the show details and every season's episode
// list from TMDB, in parallel (bounded). Performed OUTSIDE any DB tx.
func (s *LibraryServiceImpl) fetchShowCatalog(ctx context.Context, tmdbID int) (*preloadedShow, error) {
	details, err := s.tmdb.GetTV(ctx, tmdbID)
	if err != nil {
		return nil, fmt.Errorf("fetching TV show from TMDB: %w", err)
	}

	const maxConcurrent = 6
	sem := make(chan struct{}, maxConcurrent)
	var (
		mu       sync.Mutex
		seasons  = make(map[int]*tmdb.SeasonDetails, len(details.Seasons))
		firstErr error
		wg       sync.WaitGroup
	)
	for _, ts := range details.Seasons {
		wg.Add(1)
		sem <- struct{}{}
		go func(seasonNumber int) {
			defer wg.Done()
			defer func() { <-sem }()
			sd, err := s.tmdb.GetSeason(ctx, tmdbID, seasonNumber)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("fetching season %d: %w", seasonNumber, err)
				}
				return
			}
			seasons[seasonNumber] = sd
		}(ts.SeasonNumber)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return &preloadedShow{show: details, seasons: seasons}, nil
}

// persistShowCatalog writes a preloaded TMDB show + all its seasons and
// episodes to the catalog tables. Runs entirely against the DB — no HTTP
// — and is safe to call inside a write transaction.
func (s *LibraryServiceImpl) persistShowCatalog(ctx context.Context, pre *preloadedShow) (*model.TMDBShow, error) {
	if pre == nil || pre.show == nil {
		return nil, fmt.Errorf("preloaded show is nil")
	}
	details := pre.show
	show := &model.TMDBShow{
		TMDBID:       details.ID,
		Title:        details.Name,
		Overview:     details.Overview,
		PosterPath:   details.PosterPath,
		BackdropPath: details.BackdropPath,
		FirstAirDate: parseTMDBDate(details.FirstAirDate),
		Genres:       genreNames(details.Genres),
		TMDBStatus:   details.Status,
		RefreshedAt:  s.now(),
	}
	if err := s.shows.Upsert(ctx, show); err != nil {
		return nil, err
	}

	// Seasons first so we have IDs for episode FKs.
	var seasons []model.TMDBSeason
	for _, ts := range details.Seasons {
		seasons = append(seasons, model.TMDBSeason{
			ShowID:       show.ID,
			TMDBSeasonID: ts.ID,
			SeasonNumber: ts.SeasonNumber,
			Name:         ts.Name,
			Overview:     ts.Overview,
			PosterPath:   ts.PosterPath,
			AirDate:      parseTMDBDate(ts.AirDate),
			EpisodeCount: ts.EpisodeCount,
		})
	}
	if err := s.seasons.UpsertBatch(ctx, seasons); err != nil {
		return nil, err
	}
	persistedSeasons, err := s.seasons.ListByShow(ctx, show.ID)
	if err != nil {
		return nil, err
	}
	seasonIDByNumber := make(map[int]string, len(persistedSeasons))
	for _, ps := range persistedSeasons {
		seasonIDByNumber[ps.SeasonNumber] = ps.ID
	}

	for seasonNumber, sd := range pre.seasons {
		seasonID, ok := seasonIDByNumber[seasonNumber]
		if !ok {
			continue
		}
		var eps []model.TMDBEpisode
		for _, te := range sd.Episodes {
			eps = append(eps, model.TMDBEpisode{
				SeasonID:       seasonID,
				TMDBEpisodeID:  te.ID,
				EpisodeNumber:  te.EpisodeNumber,
				Name:           te.Name,
				Overview:       te.Overview,
				StillPath:      te.StillPath,
				AirDate:        parseTMDBDate(te.AirDate),
				RuntimeMinutes: te.Runtime,
			})
		}
		if len(eps) > 0 {
			if err := s.episodes.UpsertBatch(ctx, eps); err != nil {
				return nil, err
			}
		}
	}

	return show, nil
}

func (s *LibraryServiceImpl) Remove(ctx context.Context, id string) error {
	return s.library.Delete(ctx, id)
}

func (s *LibraryServiceImpl) SetStatus(ctx context.Context, id string, status *model.MediaStatus) error {
	return s.library.SetManualStatus(ctx, id, status)
}

func (s *LibraryServiceImpl) UpdateNotes(ctx context.Context, id, notes string) error {
	return s.library.UpdateNotes(ctx, id, notes)
}

func (s *LibraryServiceImpl) UpdateWatchedAt(ctx context.Context, id string, watchedAt *int64) error {
	return s.library.UpdateWatchedAt(ctx, id, watchedAt)
}

// RefreshCatalog enqueues a background refresh of a show's full TMDB
// catalog. Returns immediately — the actual HTTP + DB work runs in the
// TMDBJobRunner. If the runner isn't wired (tests), it falls back to
// running the fetch inline.
func (s *LibraryServiceImpl) RefreshCatalog(ctx context.Context, tmdbShowID string) error {
	show, err := s.shows.GetByID(ctx, tmdbShowID)
	if err != nil {
		return err
	}
	if s.tmdbJobs != nil {
		_, err := s.tmdbJobs.EnqueueRefreshCatalog(ctx, show.TMDBID, string(model.MediaTypeTV))
		return err
	}
	return s.CompleteCatalogFetch(ctx, show.TMDBID, string(model.MediaTypeTV))
}

// RefreshAll refreshes every show in the catalog that isn't flagged as
// terminal (ended / canceled). Used by the metadata_sync job.
func (s *LibraryServiceImpl) RefreshAll(ctx context.Context) error {
	shows, err := s.shows.ListByTerminalStatus(ctx, false)
	if err != nil {
		return err
	}
	for _, sh := range shows {
		if err := s.RefreshCatalog(ctx, sh.ID); err != nil {
			return fmt.Errorf("refresh %s: %w", sh.Title, err)
		}
	}
	return nil
}

// parseTMDBDate converts a TMDB "YYYY-MM-DD" string to a unix-seconds
// pointer. Empty strings and unparseable inputs return nil.
func parseTMDBDate(s string) *int64 {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	v := t.UTC().Unix()
	return &v
}
