// Calendar, discover, suggestions, keywords, and menu pages.
package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

func (h *PageHandler) Calendar(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	filter := model.CalendarFilter{
		MediaType: model.MediaType(q.Get("type")),
		Status:    model.MediaStatus(q.Get("status")),
		Range:     q.Get("range"),
	}
	if filter.Range == "" {
		filter.Range = "7d"
	}
	upcoming, _ := h.calendar.Upcoming(r.Context(), userID, filter)
	recent, _ := h.calendar.RecentlyReleased(r.Context(), userID, filter)
	data := h.baseData("calendar", user, settings)
	data["Upcoming"] = upcoming
	data["Recent"] = recent
	data["Filter"] = filter
	data["Section"] = q.Get("section")
	h.render(w, "calendar", r, data)
}

// --- Discover ---

func (h *PageHandler) Discover(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	tab := q.Get("tab")
	if tab == "" {
		tab = "trending"
	}
	data := h.baseData("discover", user, settings)
	data["Tab"] = tab
	data["Msg"] = q.Get("msg")

	switch tab {
	case "trending":
		trendingType := q.Get("type")
		if trendingType == "" {
			trendingType = "movie"
		}
		result, err := h.discovery.Trending(r.Context())
		if err != nil {
			slog.Error("trending", "error", err)
		}
		if result != nil {
			var items []model.DiscoverItem
			if trendingType == "tv" {
				items = result.TV
			} else {
				items = result.Movies
			}
			data["TrendingItems"] = h.enrichDiscoverItems(r.Context(), userID, items)
		}
		data["TrendingType"] = trendingType
	case "popular":
		mediaType := model.MediaType(q.Get("type"))
		if mediaType == "" {
			mediaType = model.MediaTypeMovie
		}
		page, _ := strconv.Atoi(q.Get("page"))
		if page < 1 {
			page = 1
		}
		result, err := h.discovery.Popular(r.Context(), mediaType, page)
		if err != nil {
			slog.Error("popular", "error", err)
		}
		if result != nil {
			result.Items = h.enrichDiscoverItems(r.Context(), userID, result.Items)
		}
		data["Popular"] = result
		data["PopularType"] = mediaType
		data["PopularPage"] = page
	case "recommendations":
		recs, err := h.discovery.Recommendations(r.Context(), userID)
		if err != nil {
			slog.Error("recommendations", "error", err)
		}
		if len(recs) > 0 {
			data["NextRecommendation"] = recs[0]
			data["RemainingCount"] = len(recs) - 1
		}
	}
	h.render(w, "discover", r, data)
}

func (h *PageHandler) HandleDiscoverAdd(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	tmdbID, _ := strconv.Atoi(r.FormValue("tmdb_id"))
	mediaType := model.MediaType(r.FormValue("media_type"))
	returnTo := r.FormValue("return_to")
	if _, err := h.library.AddWithStub(r.Context(), userID, tmdbID, mediaType, readAddStub(r)); err != nil {
		slog.Error("add from discover", "error", err)
	}
	if returnTo == "" {
		returnTo = "/discover"
	}
	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (h *PageHandler) HandleDismissRecommendation(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	tmdbID, _ := strconv.Atoi(r.FormValue("tmdb_id"))
	mediaType := r.FormValue("media_type")
	if err := h.dismissedRepo.Dismiss(r.Context(), userID, tmdbID, mediaType); err != nil {
		slog.Error("dismiss recommendation", "error", err)
	}
	http.Redirect(w, r, "/discover?tab=recommendations", http.StatusSeeOther)
}

// --- Suggestions ---

func (h *PageHandler) Suggestions(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	suggestions, _ := h.keywords.ListSuggestions(r.Context(), userID)
	watches, _ := h.keywords.List(r.Context(), userID)
	showDismissed := r.URL.Query().Get("show") == "dismissed"
	var dismissed []model.KeywordResult
	if showDismissed {
		dismissed, _ = h.keywords.ListDismissed(r.Context(), userID)
	}
	kwNames := map[string]string{}
	for _, kw := range watches {
		kwNames[kw.ID] = kw.Keyword
	}
	data := h.baseData("suggestions", user, settings)
	data["Suggestions"] = suggestions
	data["Dismissed"] = dismissed
	data["ShowDismissed"] = showDismissed
	data["KeywordNames"] = kwNames
	h.render(w, "suggestions", r, data)
}

func (h *PageHandler) HandleSuggestionAdd(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	resultID := chi.URLParam(r, "id")
	tmdbID, _ := strconv.Atoi(r.FormValue("tmdb_id"))
	mediaType := model.MediaType(r.FormValue("media_type"))
	if tmdbID > 0 {
		if _, err := h.library.AddWithStub(r.Context(), userID, tmdbID, mediaType, readAddStub(r)); err != nil {
			slog.Error("add suggestion to library", "error", err)
		}
	}
	if err := h.keywords.AddSuggestion(r.Context(), resultID); err != nil {
		slog.Error("mark suggestion added", "error", err)
	}
	http.Redirect(w, r, "/suggestions", http.StatusSeeOther)
}

func (h *PageHandler) HandleSuggestionDismiss(w http.ResponseWriter, r *http.Request) {
	resultID := chi.URLParam(r, "id")
	_ = h.keywords.DismissSuggestion(r.Context(), resultID)
	http.Redirect(w, r, "/suggestions", http.StatusSeeOther)
}

func (h *PageHandler) HandleSuggestionDismissAll(w http.ResponseWriter, r *http.Request) {
	kwID := chi.URLParam(r, "kwId")
	_ = h.keywords.DismissAll(r.Context(), kwID)
	http.Redirect(w, r, "/suggestions", http.StatusSeeOther)
}

func (h *PageHandler) HandleSuggestionRestore(w http.ResponseWriter, r *http.Request) {
	resultID := chi.URLParam(r, "id")
	_ = h.keywords.RestoreSuggestion(r.Context(), resultID)
	http.Redirect(w, r, "/suggestions?show=dismissed", http.StatusSeeOther)
}

// --- Keywords ---

func (h *PageHandler) Keywords(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	watches, _ := h.keywords.List(r.Context(), userID)
	data := h.baseData("keywords", user, settings)
	data["Watches"] = watches
	data["Msg"] = r.URL.Query().Get("msg")
	h.render(w, "keywords", r, data)
}

func (h *PageHandler) KeywordNewPage(w http.ResponseWriter, r *http.Request) {
	_, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	h.render(w, "keyword_new", r, h.baseData("keywords", user, settings))
}

func (h *PageHandler) KeywordEditPage(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	kwID := chi.URLParam(r, "id")
	watches, _ := h.keywords.List(r.Context(), userID)
	var target *model.KeywordWatch
	for i := range watches {
		if watches[i].ID == kwID {
			target = &watches[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	data := h.baseData("keywords", user, settings)
	data["Watch"] = target
	h.render(w, "keyword_edit", r, data)
}

func (h *PageHandler) HandleKeywordCreate(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	keyword := r.FormValue("keyword")
	mediaTypes := r.FormValue("media_types")
	if _, err := h.keywords.Create(r.Context(), userID, keyword, mediaTypes); err != nil {
		slog.Error("create keyword", "error", err)
	}
	http.Redirect(w, r, "/keywords", http.StatusSeeOther)
}

func (h *PageHandler) HandleKeywordUpdate(w http.ResponseWriter, r *http.Request) {
	kwID := chi.URLParam(r, "id")
	kw := &model.KeywordWatch{
		Keyword:    r.FormValue("keyword"),
		MediaTypes: r.FormValue("media_types"),
	}
	_ = h.keywords.Update(r.Context(), kwID, []string{"keyword", "media_types"}, kw)
	http.Redirect(w, r, "/keywords", http.StatusSeeOther)
}

func (h *PageHandler) HandleKeywordDelete(w http.ResponseWriter, r *http.Request) {
	kwID := chi.URLParam(r, "id")
	_ = h.keywords.Delete(r.Context(), kwID)
	http.Redirect(w, r, "/keywords", http.StatusSeeOther)
}

// --- Menu ---

func (h *PageHandler) Menu(w http.ResponseWriter, r *http.Request) {
	_, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	h.render(w, "menu", r, h.baseData("menu", user, settings))
}
