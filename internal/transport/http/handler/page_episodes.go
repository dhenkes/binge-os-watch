// Episode, season, and watch-tracking pages.
package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/dhenkes/binge-os-watch/internal/transport/http/middleware"
)

func (h *PageHandler) HandleWatchNext(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	libraryID := chi.URLParam(r, "id")
	v, err := h.libraryRepo.GetByID(r.Context(), libraryID)
	if err == nil && v != nil && v.Show != nil {
		if _, err := h.watch.WatchNext(r.Context(), userID, v.Show.ID); err != nil {
			slog.Error("watch next", "error", err)
		}
	}
	http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
}

// --- Episode mutators ---

func (h *PageHandler) HandleWatchEpisode(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	episodeID := chi.URLParam(r, "id")
	if err := h.watch.WatchEpisode(r.Context(), userID, episodeID, 0, ""); err != nil {
		slog.Error("watch episode", "error", err)
	}
	h.redirectAfterEpisode(w, r, episodeID, r.FormValue("return_to"))
}

func (h *PageHandler) HandleUnwatchEpisode(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	episodeID := chi.URLParam(r, "id")
	if err := h.watch.UnwatchAllForEpisode(r.Context(), userID, episodeID); err != nil {
		slog.Error("unwatch episode", "error", err)
	}
	h.redirectAfterEpisode(w, r, episodeID, r.FormValue("return_to"))
}

func (h *PageHandler) HandleRateEpisode(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	episodeID := chi.URLParam(r, "id")
	score, _ := strconv.Atoi(r.FormValue("score"))
	_ = h.ratings.RateEpisode(r.Context(), userID, episodeID, score)
	h.redirectAfterEpisode(w, r, episodeID, r.FormValue("return_to"))
}

func (h *PageHandler) HandleWatchAllSeason(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	seasonID := chi.URLParam(r, "id")
	if err := h.watch.WatchAllInSeason(r.Context(), userID, seasonID); err != nil {
		slog.Error("watch all season", "error", err)
	}
	http.Redirect(w, r, r.FormValue("return_to"), http.StatusSeeOther)
}

func (h *PageHandler) HandleRateSeason(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	seasonID := chi.URLParam(r, "id")
	score, _ := strconv.Atoi(r.FormValue("score"))
	_ = h.ratings.RateSeason(r.Context(), userID, seasonID, score)
	http.Redirect(w, r, r.FormValue("return_to"), http.StatusSeeOther)
}

func (h *PageHandler) HandleWatchUpTo(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	episodeID := chi.URLParam(r, "id")
	if err := h.watch.WatchUpToEpisode(r.Context(), userID, episodeID); err != nil {
		slog.Error("watch up to", "error", err)
	}
	h.redirectAfterEpisode(w, r, episodeID, r.FormValue("return_to"))
}

func (h *PageHandler) HandleUnwatchUpTo(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	episodeID := chi.URLParam(r, "id")
	if err := h.watch.UnwatchUpToEpisode(r.Context(), userID, episodeID); err != nil {
		slog.Error("unwatch up to", "error", err)
	}
	h.redirectAfterEpisode(w, r, episodeID, r.FormValue("return_to"))
}

// HandleWatchUpToDate is the "mark up to here with a chosen date mode"
// affordance on the media detail page. Supported modes: "today" (default),
// "release" (each event stamped with its episode's air date), and "custom"
// (a single user-picked date applied to every inserted event).
func (h *PageHandler) HandleWatchUpToDate(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	libraryID := chi.URLParam(r, "id")
	episodeID := r.FormValue("season_episode")
	if episodeID == "" {
		http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
		return
	}
	mode := r.FormValue("date_mode")
	var customAt int64
	if mode == "custom" {
		if t, err := time.Parse("2006-01-02", r.FormValue("custom_date")); err == nil {
			customAt = t.UTC().Unix()
		} else {
			// Fall back to today when the user picked "custom" but the
			// date field was blank or malformed.
			mode = "today"
		}
	}
	if err := h.watch.WatchUpToEpisodeWithDate(r.Context(), userID, episodeID, mode, customAt); err != nil {
		slog.Error("watch up to date", "error", err)
	}
	http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
}

func (h *PageHandler) Watched(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	mediaType := q.Get("type")
	sortBy := q.Get("sort")
	if sortBy == "" {
		sortBy = "watched_at"
	}
	filter := model.LibraryFilter{
		Statuses: []model.MediaStatus{model.MediaStatusWatched, model.MediaStatusWatching},
		SortBy:   sortBy,
	}
	if mediaType != "" {
		filter.MediaType = model.MediaType(mediaType)
	}
	page := model.PageRequest{PageSize: 50, PageToken: q.Get("page_token")}
	result, err := h.libraryRepo.List(r.Context(), userID, filter, page)
	if err != nil {
		slog.Error("listing watched", "error", err)
		result = &model.PageResponse[model.LibraryView]{}
	}
	result.EnsureItems()

	// Flatten views to cards so the template stays simple, and attach
	// ratings via the per-subject repo.
	type WatchedItem struct {
		Media  LibraryCard
		Rating int
	}
	var showIDs, movieIDs []string
	for _, v := range result.Items {
		if v.Show != nil {
			showIDs = append(showIDs, v.Show.ID)
		}
		if v.Movie != nil {
			movieIDs = append(movieIDs, v.Movie.ID)
		}
	}
	showRatings, _ := h.ratingRepo.ListShowRatingsByIDs(r.Context(), userID, showIDs)
	movieRatings, _ := h.ratingRepo.ListMovieRatingsByIDs(r.Context(), userID, movieIDs)

	items := make([]WatchedItem, len(result.Items))
	for i, v := range result.Items {
		card := toCard(v)
		var rating int
		if v.Show != nil {
			rating = showRatings[v.Show.ID]
		} else if v.Movie != nil {
			rating = movieRatings[v.Movie.ID]
		}
		items[i] = WatchedItem{Media: card, Rating: rating}
	}
	if sortBy == "rating" {
		for i := 0; i < len(items); i++ {
			for j := i + 1; j < len(items); j++ {
				if items[j].Rating > items[i].Rating {
					items[i], items[j] = items[j], items[i]
				}
			}
		}
	}
	data := h.baseData("watched", user, settings)
	data["Items"] = items
	data["TotalSize"] = result.TotalSize
	data["NextPageToken"] = result.NextPageToken
	data["MediaType"] = mediaType
	data["Sort"] = sortBy
	h.render(w, "watched", r, data)
}

// --- Episode detail ---

func (h *PageHandler) EpisodeDetail(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	episodeID := chi.URLParam(r, "id")
	ep, err := h.episodes.GetByID(r.Context(), episodeID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	watched, _ := h.events.HasEpisode(r.Context(), userID, episodeID)
	latest, _ := h.events.LatestForEpisode(r.Context(), userID, episodeID)
	season, _ := h.seasons.GetByID(r.Context(), ep.SeasonID)

	type EpisodeView struct {
		ID             string
		EpisodeNumber  int
		Name           string
		Overview       string
		StillPath      string
		AirDate        *int64
		RuntimeMinutes int
		Watched        bool
		WatchedAt      *int64
		Notes          string
		UserRating     int
	}
	ev := EpisodeView{
		ID:             ep.ID,
		EpisodeNumber:  ep.EpisodeNumber,
		Name:           ep.Name,
		Overview:       ep.Overview,
		StillPath:      ep.StillPath,
		AirDate:        ep.AirDate,
		RuntimeMinutes: ep.RuntimeMinutes,
		Watched:        watched,
	}
	if latest != nil {
		v := latest.WatchedAt
		ev.WatchedAt = &v
		ev.Notes = latest.Notes
	}
	if rating, _ := h.ratingRepo.GetEpisode(r.Context(), userID, episodeID); rating != nil {
		ev.UserRating = rating.Score
	}

	var seasonNumber int
	var libraryID string
	var mediaTitle string
	if season != nil {
		seasonNumber = season.SeasonNumber
		if lv, err := h.libraryRepo.GetByShow(r.Context(), userID, season.ShowID); err == nil && lv != nil {
			libraryID = lv.Entry.ID
			if lv.Show != nil {
				mediaTitle = lv.Show.Title
			}
		}
	}
	data := h.baseData("library", user, settings)
	data["Episode"] = ev
	data["SeasonNumber"] = seasonNumber
	data["MediaID"] = libraryID
	data["MediaTitle"] = mediaTitle
	h.render(w, "episode", r, data)
}

func (h *PageHandler) HandleEpisodeNotes(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	episodeID := chi.URLParam(r, "id")
	notes := r.FormValue("notes")
	// Notes live on the watch_event row under Option B. Try to update the
	// most recent event first; if the user has never marked the episode
	// watched, stamp one at "now" so the note has somewhere to live.
	updated, err := h.events.UpdateLatestNotesForEpisode(r.Context(), userID, episodeID, notes)
	if err != nil {
		slog.Error("update episode notes", "error", err)
	} else if !updated {
		if err := h.watch.WatchEpisode(r.Context(), userID, episodeID, 0, notes); err != nil {
			slog.Error("create episode watch for notes", "error", err)
		}
	}
	http.Redirect(w, r, "/episodes/"+episodeID, http.StatusSeeOther)
}

func (h *PageHandler) HandleEpisodeWatchedAt(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	episodeID := chi.URLParam(r, "id")
	dateStr := r.FormValue("watched_at")
	if dateStr != "" {
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			_ = h.watch.WatchEpisode(r.Context(), userID, episodeID, t.UTC().Unix(), "")
		}
	}
	http.Redirect(w, r, "/episodes/"+episodeID, http.StatusSeeOther)
}

func (h *PageHandler) redirectAfterEpisode(w http.ResponseWriter, r *http.Request, episodeID, returnTo string) {
	if returnTo == "" {
		if ep, err := h.episodes.GetByID(r.Context(), episodeID); err == nil {
			if season, err := h.seasons.GetByID(r.Context(), ep.SeasonID); err == nil && season != nil {
				if lv, err := h.libraryRepo.GetByShow(r.Context(), middleware.UserIDFromContext(r.Context()), season.ShowID); err == nil && lv != nil {
					returnTo = "/media/" + lv.Entry.ID
				}
			}
		}
		if returnTo == "" {
			returnTo = "/"
		}
	}
	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}
