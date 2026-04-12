package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/dhenkes/binge-os-watch/internal/tmdb"
)

type DiscoveryServiceImpl struct {
	tmdb      *tmdb.Client
	library   model.LibraryRepository
	dismissed model.DismissedRecommendationRepository

	// In-memory caches with TTL.
	trendingCache    *cachedValue[*model.TrendingResult]
	recommendCache   map[string]*cachedValue[[]model.RecommendationItem]
	recommendCacheMu sync.Mutex

	// Per-media detail caches — the media detail page re-fetches both on
	// every load, so caching these removes hundreds of ms of TMDB latency.
	providersCache   map[string]*cachedValue[*model.WatchProviderResult] // key: mediaID|region
	providersCacheMu sync.Mutex
	mediaRecsCache   map[string]*cachedValue[[]model.RecommendationItem] // key: mediaID
	mediaRecsCacheMu sync.Mutex
}

var _ model.DiscoveryService = (*DiscoveryServiceImpl)(nil)

func NewDiscoveryService(tmdbClient *tmdb.Client, library model.LibraryRepository, dismissed model.DismissedRecommendationRepository) *DiscoveryServiceImpl {
	return &DiscoveryServiceImpl{
		tmdb:           tmdbClient,
		library:        library,
		dismissed:      dismissed,
		trendingCache:  newCachedValue[*model.TrendingResult](6 * time.Hour),
		recommendCache: make(map[string]*cachedValue[[]model.RecommendationItem]),
		providersCache: make(map[string]*cachedValue[*model.WatchProviderResult]),
		mediaRecsCache: make(map[string]*cachedValue[[]model.RecommendationItem]),
	}
}

// ClearRecommendationCache invalidates cached recommendations for a user.
func (s *DiscoveryServiceImpl) ClearRecommendationCache(userID string) {
	s.recommendCacheMu.Lock()
	delete(s.recommendCache, userID)
	s.recommendCacheMu.Unlock()
}

func (s *DiscoveryServiceImpl) Trending(ctx context.Context) (*model.TrendingResult, error) {
	if v, ok := s.trendingCache.get(); ok {
		return v, nil
	}

	movies, err := s.tmdb.TrendingMovies(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching trending movies: %w", err)
	}
	tv, err := s.tmdb.TrendingTV(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching trending TV: %w", err)
	}

	result := &model.TrendingResult{
		Movies: toDiscoverItems(movies.Results),
		TV:     toDiscoverItems(tv.Results),
	}
	s.trendingCache.set(result)
	return result, nil
}

func (s *DiscoveryServiceImpl) Popular(ctx context.Context, mediaType model.MediaType, page int) (*model.PopularResult, error) {
	if page < 1 {
		page = 1
	}

	var resp *tmdb.SearchResponse
	var err error
	switch mediaType {
	case model.MediaTypeTV:
		resp, err = s.tmdb.PopularTV(ctx, page)
	default:
		resp, err = s.tmdb.PopularMovies(ctx, page)
	}
	if err != nil {
		return nil, fmt.Errorf("fetching popular: %w", err)
	}

	return &model.PopularResult{
		Items:      toDiscoverItems(resp.Results),
		Page:       resp.Page,
		TotalPages: resp.TotalPages,
	}, nil
}

func (s *DiscoveryServiceImpl) Recommendations(ctx context.Context, userID string) ([]model.RecommendationItem, error) {
	s.recommendCacheMu.Lock()
	cache, exists := s.recommendCache[userID]
	if !exists {
		cache = newCachedValue[[]model.RecommendationItem](24 * time.Hour)
		s.recommendCache[userID] = cache
	}
	s.recommendCacheMu.Unlock()

	// Cache holds ALL TMDB recommendations (unfiltered). Filtering for
	// library/dismissed is done per-request so it's always current without
	// needing to clear the cache.
	var allRecs []model.RecommendationItem
	if v, ok := cache.get(); ok {
		allRecs = v
	} else {
		// Get user's top 10 rated library entries (shows + movies).
		top, err := s.library.ListTopRatedEntries(ctx, userID, 10)
		if err != nil {
			return nil, err
		}

		type recKey struct {
			tmdbID    int
			mediaType string
		}
		freq := make(map[recKey]*model.RecommendationItem)

		for _, v := range top {
			var recs *tmdb.RecommendationResponse
			switch v.Entry.MediaType {
			case model.MediaTypeMovie:
				if v.Movie == nil {
					continue
				}
				recs, err = s.tmdb.GetMovieRecommendations(ctx, v.Movie.TMDBID)
			case model.MediaTypeTV:
				if v.Show == nil {
					continue
				}
				recs, err = s.tmdb.GetTVRecommendations(ctx, v.Show.TMDBID)
			}
			if err != nil || recs == nil {
				continue
			}

			for _, r := range recs.Results {
				key := recKey{tmdbID: r.ID, mediaType: r.MediaType}
				if item, ok := freq[key]; ok {
					item.Count++
				} else {
					freq[key] = &model.RecommendationItem{
						TMDBID:      r.ID,
						MediaType:   r.MediaType,
						Title:       r.DisplayTitle(),
						Overview:    r.Overview,
						PosterPath:  r.PosterPath,
						ReleaseDate: r.ReleaseDate,
						VoteAverage: r.VoteAverage,
						Count:       1,
					}
					if freq[key].ReleaseDate == "" {
						freq[key].ReleaseDate = r.FirstAirDate
					}
				}
			}
		}

		for _, item := range freq {
			allRecs = append(allRecs, *item)
		}
		sortRecommendations(allRecs)
		cache.set(allRecs)
	}

	// Filter out in-library and dismissed items (fast DB lookups, no TMDB).
	libMap, _ := s.library.GetLibraryMap(ctx, userID)
	dismissedSet, _ := s.dismissed.ListAll(ctx, userID)

	var results []model.RecommendationItem
	for _, item := range allRecs {
		key := fmt.Sprintf("%d:%s", item.TMDBID, item.MediaType)
		if libMap[key] != "" || dismissedSet[key] {
			continue
		}
		results = append(results, item)
	}

	// Fallback: if the user has no ratings yet (or every rec was already
	// in-library or dismissed), surface trending as a "cold start" feed so
	// the recommendations tab is never empty on day one.
	if len(results) == 0 {
		if trending, err := s.Trending(ctx); err == nil && trending != nil {
			pool := make([]model.DiscoverItem, 0, len(trending.Movies)+len(trending.TV))
			pool = append(pool, trending.Movies...)
			pool = append(pool, trending.TV...)
			for _, d := range pool {
				key := fmt.Sprintf("%d:%s", d.TMDBID, d.MediaType)
				if libMap[key] != "" || dismissedSet[key] {
					continue
				}
				results = append(results, model.RecommendationItem{
					TMDBID:      d.TMDBID,
					MediaType:   d.MediaType,
					Title:       d.Title,
					Overview:    d.Overview,
					PosterPath:  d.PosterPath,
					ReleaseDate: d.ReleaseDate,
					VoteAverage: d.VoteAverage,
					Count:       0,
				})
			}
		}
	}

	return results, nil
}

func (s *DiscoveryServiceImpl) WatchProviders(ctx context.Context, libraryID, region string) (*model.WatchProviderResult, error) {
	key := libraryID + "|" + region
	s.providersCacheMu.Lock()
	entry, ok := s.providersCache[key]
	if !ok {
		entry = newCachedValue[*model.WatchProviderResult](24 * time.Hour)
		s.providersCache[key] = entry
	}
	s.providersCacheMu.Unlock()

	if v, ok := entry.get(); ok {
		return v, nil
	}

	v, err := s.library.GetByID(ctx, libraryID)
	if err != nil {
		return nil, err
	}

	var providers *tmdb.WatchProviders
	switch v.Entry.MediaType {
	case model.MediaTypeMovie:
		if v.Movie == nil {
			return &model.WatchProviderResult{}, nil
		}
		providers, err = s.tmdb.GetMovieWatchProviders(ctx, v.Movie.TMDBID, region)
	case model.MediaTypeTV:
		if v.Show == nil {
			return &model.WatchProviderResult{}, nil
		}
		providers, err = s.tmdb.GetTVWatchProviders(ctx, v.Show.TMDBID, region)
	}
	if err != nil {
		return nil, fmt.Errorf("fetching watch providers: %w", err)
	}

	result := &model.WatchProviderResult{
		Stream: toProviderInfos(providers.Stream),
		Rent:   toProviderInfos(providers.Rent),
		Buy:    toProviderInfos(providers.Buy),
		Link:   providers.Link,
	}
	entry.set(result)
	return result, nil
}

func (s *DiscoveryServiceImpl) MediaRecommendations(ctx context.Context, libraryID string) ([]model.RecommendationItem, error) {
	s.mediaRecsCacheMu.Lock()
	entry, ok := s.mediaRecsCache[libraryID]
	if !ok {
		entry = newCachedValue[[]model.RecommendationItem](6 * time.Hour)
		s.mediaRecsCache[libraryID] = entry
	}
	s.mediaRecsCacheMu.Unlock()

	if v, ok := entry.get(); ok {
		return v, nil
	}

	v, err := s.library.GetByID(ctx, libraryID)
	if err != nil {
		return nil, err
	}

	var recs *tmdb.RecommendationResponse
	switch v.Entry.MediaType {
	case model.MediaTypeMovie:
		if v.Movie == nil {
			return nil, nil
		}
		recs, err = s.tmdb.GetMovieRecommendations(ctx, v.Movie.TMDBID)
	case model.MediaTypeTV:
		if v.Show == nil {
			return nil, nil
		}
		recs, err = s.tmdb.GetTVRecommendations(ctx, v.Show.TMDBID)
	}
	if err != nil {
		return nil, fmt.Errorf("fetching recommendations: %w", err)
	}

	var items []model.RecommendationItem
	for _, r := range recs.Results {
		releaseDate := r.ReleaseDate
		if releaseDate == "" {
			releaseDate = r.FirstAirDate
		}
		items = append(items, model.RecommendationItem{
			TMDBID:      r.ID,
			MediaType:   r.MediaType,
			Title:       r.DisplayTitle(),
			PosterPath:  r.PosterPath,
			ReleaseDate: releaseDate,
			VoteAverage: r.VoteAverage,
			Count:       1,
		})
	}
	entry.set(items)
	return items, nil
}

func toDiscoverItems(results []tmdb.SearchResult) []model.DiscoverItem {
	items := make([]model.DiscoverItem, len(results))
	for i, r := range results {
		releaseDate := r.ReleaseDate
		if releaseDate == "" {
			releaseDate = r.FirstAirDate
		}
		items[i] = model.DiscoverItem{
			TMDBID:      r.ID,
			MediaType:   r.MediaType,
			Title:       r.DisplayTitle(),
			Overview:    r.Overview,
			PosterPath:  r.PosterPath,
			ReleaseDate: releaseDate,
			VoteAverage: r.VoteAverage,
		}
	}
	return items
}

func toProviderInfos(providers []tmdb.Provider) []model.ProviderInfo {
	if len(providers) == 0 {
		return nil
	}
	infos := make([]model.ProviderInfo, len(providers))
	for i, p := range providers {
		infos[i] = model.ProviderInfo{
			ProviderID:   p.ProviderID,
			ProviderName: p.ProviderName,
			LogoPath:     p.LogoPath,
		}
	}
	return infos
}

func sortRecommendations(items []model.RecommendationItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0; j-- {
			if items[j].Count > items[j-1].Count ||
				(items[j].Count == items[j-1].Count && items[j].VoteAverage > items[j-1].VoteAverage) {
				items[j], items[j-1] = items[j-1], items[j]
			} else {
				break
			}
		}
	}
}

// cachedValue is a simple in-memory cache with TTL.
type cachedValue[T any] struct {
	mu    sync.RWMutex
	value T
	valid bool
	ttl   time.Duration
	setAt time.Time
}

func newCachedValue[T any](ttl time.Duration) *cachedValue[T] {
	return &cachedValue[T]{ttl: ttl}
}

func (c *cachedValue[T]) get() (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.valid && time.Since(c.setAt) < c.ttl {
		return c.value, true
	}
	var zero T
	return zero, false
}

func (c *cachedValue[T]) set(v T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = v
	c.valid = true
	c.setAt = time.Now()
}
