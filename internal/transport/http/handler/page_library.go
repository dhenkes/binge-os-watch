// Library and media detail pages.
package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// --- Library ---

func (h *PageHandler) Library(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	q := r.URL.Query()
	view := q.Get("view")

	data := h.baseData("library", user, settings)

	if view == "all" {
		filter := model.LibraryFilter{
			Status:    model.MediaStatus(q.Get("status")),
			MediaType: model.MediaType(q.Get("type")),
			TagID:     q.Get("tag"),
			SortBy:    q.Get("sort"),
			SortDir:   q.Get("dir"),
		}
		page := model.PageRequest{PageSize: 40, PageToken: q.Get("page_token")}
		result, err := h.libraryRepo.List(r.Context(), userID, filter, page)
		if err != nil {
			slog.Error("listing library", "error", err)
			result = &model.PageResponse[model.LibraryView]{}
		}
		result.EnsureItems()

		items := toCards(result.Items)
		data["View"] = "all"
		data["Items"] = items
		data["NextPageToken"] = result.NextPageToken
		data["TotalSize"] = result.TotalSize
		data["Filter"] = filter
		data["AutoRefresh"] = anyRefreshing(items)
	} else {
		stats, _ := h.stats.GetUserStats(r.Context(), userID)
		upcoming, _ := h.calendar.Upcoming(r.Context(), userID, model.CalendarFilter{Range: "30d"})
		if len(upcoming) > 5 {
			upcoming = upcoming[:5]
		}
		watching, _ := h.libraryRepo.ListContinueWatching(r.Context(), userID, 10)
		unrated, _ := h.libraryRepo.ListUnratedWatched(r.Context(), userID, 10)
		totalSize, _ := h.libraryRepo.TotalCount(r.Context(), userID)

		watchingCards := toCards(watching)
		unratedCards := toCards(unrated)
		data["View"] = ""
		data["Stats"] = stats
		data["Upcoming"] = upcoming
		data["Watching"] = watchingCards
		data["Unrated"] = unratedCards
		data["AutoRefresh"] = anyRefreshing(watchingCards) || anyRefreshing(unratedCards)
		data["TotalSize"] = totalSize
	}

	h.render(w, "library", r, data)
}

// --- Media detail ---

func (h *PageHandler) MediaDetail(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	libraryID := chi.URLParam(r, "id")
	v, err := h.libraryRepo.GetByID(r.Context(), libraryID)
	if err != nil || v == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	card := toCard(*v)
	data := h.baseData("library", user, settings)
	data["Media"] = card
	data["AutoRefresh"] = card.Refreshing

	// Rating: lookup by subject.
	var rating int
	if v.Show != nil {
		if r, err := h.ratingRepo.GetShow(r.Context(), userID, v.Show.ID); err == nil && r != nil {
			rating = r.Score
		}
	} else if v.Movie != nil {
		if r, err := h.ratingRepo.GetMovie(r.Context(), userID, v.Movie.ID); err == nil && r != nil {
			rating = r.Score
		}
	}
	data["Rating"] = rating

	// Tags + all user tags.
	mediaTags, _ := h.libraryTag.ListByLibrary(r.Context(), v.Entry.ID)
	allTags, _ := h.tags.List(r.Context(), userID)
	data["MediaTags"] = mediaTags
	data["AllTags"] = allTags

	// Watch providers + recommendations.
	region := h.region(settings)
	providers, _ := h.discovery.WatchProviders(r.Context(), v.Entry.ID, region)
	data["Providers"] = providers
	recs, _ := h.discovery.MediaRecommendations(r.Context(), v.Entry.ID)
	data["Recommendations"] = h.enrichRecommendations(r.Context(), userID, recs)

	if v.Entry.MediaType == model.MediaTypeTV && v.Show != nil {
		seasons, _ := h.seasons.ListByShow(r.Context(), v.Show.ID)
		allEps, _ := h.episodes.ListByShow(r.Context(), v.Show.ID)
		watchedMap, _ := h.events.WatchedMapForShow(r.Context(), userID, v.Show.ID)
		epScores, _ := h.ratingRepo.ListEpisodeRatingsByShow(r.Context(), userID, v.Show.ID)
		seasonScores, _ := h.ratingRepo.ListSeasonRatingsByShow(r.Context(), userID, v.Show.ID)

		type EpisodeRow struct {
			ID             string
			EpisodeNumber  int
			Name           string
			Overview       string
			AirDate        *int64
			RuntimeMinutes int
			Watched        bool
			WatchedAt      *int64
			UserRating     *int
		}
		type SeasonShape struct {
			ID           string
			SeasonNumber int
			Name         string
			WatchedCount int
			EpisodeCount int
			UserRating   *int
		}
		type SeasonWithEpisodes struct {
			Season   SeasonShape
			Episodes []EpisodeRow
		}

		episodesBySeason := map[string][]EpisodeRow{}
		for _, e := range allEps {
			row := EpisodeRow{
				ID:             e.ID,
				EpisodeNumber:  e.EpisodeNumber,
				Name:           e.Name,
				Overview:       e.Overview,
				AirDate:        e.AirDate,
				RuntimeMinutes: e.RuntimeMinutes,
			}
			if t, ok := watchedMap[e.ID]; ok {
				row.Watched = true
				tt := t
				row.WatchedAt = &tt
			}
			if score, ok := epScores[e.ID]; ok {
				s := score
				row.UserRating = &s
			}
			episodesBySeason[e.SeasonID] = append(episodesBySeason[e.SeasonID], row)
		}

		var seasonsData []SeasonWithEpisodes
		for _, s := range seasons {
			eps := episodesBySeason[s.ID]
			sh := SeasonShape{
				ID:           s.ID,
				SeasonNumber: s.SeasonNumber,
				Name:         s.Name,
				EpisodeCount: len(eps),
			}
			if score, ok := seasonScores[s.ID]; ok {
				sc := score
				sh.UserRating = &sc
			}
			for _, e := range eps {
				if e.Watched {
					sh.WatchedCount++
				}
			}
			seasonsData = append(seasonsData, SeasonWithEpisodes{Season: sh, Episodes: eps})
		}
		data["Seasons"] = seasonsData

		watched, total, _ := h.events.ProgressForShow(r.Context(), userID, v.Show.ID)
		data["WatchedCount"] = watched
		data["TotalEpisodes"] = total

		if next, err := h.events.NextUnwatched(r.Context(), userID, v.Show.ID); err == nil {
			data["NextEpisode"] = next
		}
	}

	data["Msg"] = r.URL.Query().Get("msg")
	h.render(w, "media_detail", r, data)
}

// --- Library item mutators ---

func (h *PageHandler) HandleSetStatus(w http.ResponseWriter, r *http.Request) {
	libraryID := chi.URLParam(r, "id")
	statusStr := r.FormValue("status")
	var status *model.MediaStatus
	if statusStr != "" {
		s := model.MediaStatus(statusStr)
		status = &s
	}
	if err := h.library.SetStatus(r.Context(), libraryID, status); err != nil {
		slog.Error("set status", "error", err)
	}
	http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
}

func (h *PageHandler) HandleRateMedia(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	libraryID := chi.URLParam(r, "id")
	score, _ := strconv.Atoi(r.FormValue("score"))

	v, err := h.libraryRepo.GetByID(r.Context(), libraryID)
	if err != nil || v == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	switch {
	case v.Show != nil:
		_ = h.ratings.RateShow(r.Context(), userID, v.Show.ID, score)
	case v.Movie != nil:
		_ = h.ratings.RateMovie(r.Context(), userID, v.Movie.ID, score)
	}
	http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
}

func (h *PageHandler) HandleRemoveMedia(w http.ResponseWriter, r *http.Request) {
	libraryID := chi.URLParam(r, "id")
	if err := h.library.Remove(r.Context(), libraryID); err != nil {
		slog.Error("remove media", "error", err)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *PageHandler) HandleAddTag(w http.ResponseWriter, r *http.Request) {
	libraryID := chi.URLParam(r, "id")
	tagID := r.FormValue("tag_id")
	if err := h.libraryTag.Add(r.Context(), libraryID, tagID); err != nil {
		slog.Error("add tag", "error", err)
	}
	http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
}

func (h *PageHandler) HandleRemoveTag(w http.ResponseWriter, r *http.Request) {
	libraryID := chi.URLParam(r, "id")
	tagID := chi.URLParam(r, "tagId")
	if err := h.libraryTag.Remove(r.Context(), libraryID, tagID); err != nil {
		slog.Error("remove tag", "error", err)
	}
	http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
}

// --- Library-level notes + watched date ---

func (h *PageHandler) HandleWatchedAt(w http.ResponseWriter, r *http.Request) {
	_, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	libraryID := chi.URLParam(r, "id")
	dateStr := r.FormValue("watched_at")
	if dateStr != "" {
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			v := t.UTC().Unix()
			_ = h.library.UpdateWatchedAt(r.Context(), libraryID, &v)
			// Nudge movies to "watched" status when the user sets a date.
			if lv, err := h.libraryRepo.GetByID(r.Context(), libraryID); err == nil && lv != nil && lv.Entry.MediaType == model.MediaTypeMovie {
				watched := model.MediaStatusWatched
				_ = h.library.SetStatus(r.Context(), libraryID, &watched)
			}
		}
	} else {
		_ = h.library.UpdateWatchedAt(r.Context(), libraryID, nil)
	}
	http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
}

func (h *PageHandler) HandleNotes(w http.ResponseWriter, r *http.Request) {
	_, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	libraryID := chi.URLParam(r, "id")
	notes := r.FormValue("notes")
	_ = h.library.UpdateNotes(r.Context(), libraryID, notes)
	http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
}
