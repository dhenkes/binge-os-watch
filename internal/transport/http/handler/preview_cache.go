package handler

import (
	"strconv"
	"sync"
	"time"
)

// previewCache is a small in-memory TTL cache for the preview page's
// fully-rendered data map. The preview page fires a chain of TMDB calls
// (GetTV, GetSeason ×N, WatchProviders) on every visit — a 30-minute
// cache turns the second visit into a map lookup and keeps TMDB quota
// intact even if a user is browsing back and forth through search.
//
// Scope is per-process; a restart wipes it. Good enough — the discovery
// warmer is the right tool for cross-restart persistence, not this.
type previewCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]*previewCacheEntry
}

type previewCacheEntry struct {
	data  map[string]any
	setAt time.Time
}

func newPreviewCache(ttl time.Duration) *previewCache {
	return &previewCache{
		ttl:     ttl,
		entries: make(map[string]*previewCacheEntry),
	}
}

// previewKey builds the cache key. tmdbID + mediaType identifies the
// title; region is included because watch providers are region-specific.
func previewKey(tmdbID int, mediaType, region string) string {
	return strconv.Itoa(tmdbID) + "|" + mediaType + "|" + region
}

// get returns a cached data map if one is present and still within TTL.
// The returned map is the same reference stored in the cache — callers
// must treat it as read-only. Template rendering is read-only so this
// is safe for the preview page.
func (c *previewCache) get(key string) (map[string]any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Since(e.setAt) > c.ttl {
		delete(c.entries, key)
		return nil, false
	}
	return e.data, true
}

// set stores a data map under the given key. Overwrites any previous
// entry for the same key.
func (c *previewCache) set(key string, data map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = &previewCacheEntry{data: data, setAt: time.Now()}
}
