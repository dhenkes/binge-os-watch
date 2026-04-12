package model

// This file used to hold the old per-user Media struct and its repository/
// service interfaces. Under the Option B schema those concerns moved to
// library.go (LibraryEntry / LibraryView / LibraryRepository /
// LibraryService) and tmdb_catalog.go (TMDBShow / TMDBMovie / catalog
// repositories). What remains here is the shared enumeration plus the
// pure DeriveStatus helper that both the old and new code paths used.

// MediaType distinguishes movies from TV shows.
type MediaType string

const (
	MediaTypeMovie MediaType = "movie"
	MediaTypeTV    MediaType = "tv"
)

// MediaStatus represents the watch status of a library item. On the new
// schema the auto-derived status is always one of plan_to_watch /
// watching / watched; on_hold and dropped are only ever set as a manual
// override.
type MediaStatus string

const (
	MediaStatusPlanToWatch MediaStatus = "plan_to_watch"
	MediaStatusWatching    MediaStatus = "watching"
	MediaStatusWatched     MediaStatus = "watched"
	MediaStatusOnHold      MediaStatus = "on_hold"
	MediaStatusDropped     MediaStatus = "dropped"
)

// ValidMediaStatuses is the set of valid statuses for validation.
var ValidMediaStatuses = []MediaStatus{
	MediaStatusPlanToWatch,
	MediaStatusWatching,
	MediaStatusWatched,
	MediaStatusOnHold,
	MediaStatusDropped,
}

// DeriveStatus computes the auto-derived status from watch progress.
// Pure — used by the watch service's recalc path.
//
//   - 0 watched           → plan_to_watch
//   - 1+ watched, not all → watching
//   - all watched         → watched
func DeriveStatus(totalEpisodes, watchedEpisodes int, manualOverride bool) MediaStatus {
	if manualOverride {
		return "" // caller keeps current status
	}
	if watchedEpisodes <= 0 {
		return MediaStatusPlanToWatch
	}
	if totalEpisodes > 0 && watchedEpisodes >= totalEpisodes {
		return MediaStatusWatched
	}
	return MediaStatusWatching
}
