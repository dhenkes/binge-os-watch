// Search and preview pages.
package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/dhenkes/binge-os-watch/internal/tmdb"
)

func (h *PageHandler) Search(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	data := h.baseData("search", user, settings)
	query := r.URL.Query().Get("q")
	searchType := r.URL.Query().Get("type")
	data["Query"] = query
	data["SearchType"] = searchType

	if query != "" {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		var results *tmdb.SearchResponse
		var err error
		switch searchType {
		case "movie":
			results, err = h.tmdbClient.SearchMovies(r.Context(), query, page)
		case "tv":
			results, err = h.tmdbClient.SearchTV(r.Context(), query, page)
		default:
			results, err = h.tmdbClient.SearchMulti(r.Context(), query, page)
		}
		if err != nil {
			slog.Error("search", "error", err)
		} else {
			var filtered []tmdb.SearchResult
			for _, sr := range results.Results {
				if sr.MediaType == "movie" || sr.MediaType == "tv" {
					filtered = append(filtered, sr)
				}
			}
			libMap, _ := h.libraryRepo.GetLibraryMap(r.Context(), userID)

			type SearchItem struct {
				tmdb.SearchResult
				InLibrary bool
				MediaID   string
			}
			var items []SearchItem
			for _, sr := range filtered {
				key := fmt.Sprintf("%d:%s", sr.ID, sr.MediaType)
				item := SearchItem{SearchResult: sr}
				if id, ok := libMap[key]; ok {
					item.InLibrary = true
					item.MediaID = id
				}
				items = append(items, item)
			}
			data["Results"] = items
			data["SearchPage"] = results.Page
			data["TotalPages"] = results.TotalPages
			data["TotalResults"] = results.TotalResults
		}
	}
	data["Msg"] = r.URL.Query().Get("msg")
	h.render(w, "search", r, data)
}

func (h *PageHandler) HandleSearchAdd(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	tmdbID, _ := strconv.Atoi(r.FormValue("tmdb_id"))
	mediaType := model.MediaType(r.FormValue("media_type"))
	query := r.FormValue("q")
	page := r.FormValue("page")

	if _, err := h.library.AddWithStub(r.Context(), userID, tmdbID, mediaType, readAddStub(r)); err != nil {
		slog.Error("add from search", "error", err)
	}
	redirectURL := "/search?q=" + url.QueryEscape(query)
	if page != "" && page != "1" {
		redirectURL += "&page=" + page
	}
	if st := r.FormValue("type"); st != "" {
		redirectURL += "&type=" + st
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (h *PageHandler) Preview(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	tmdbID, _ := strconv.Atoi(r.URL.Query().Get("tmdb_id"))
	mediaType := r.URL.Query().Get("type")
	query := r.URL.Query().Get("q")
	region := h.region(settings)

	// Cache only the TMDB-derived fields; per-user state (InLibrary,
	// MediaID, settings) is merged in below on every request so a
	// cached entry can't leak state across users.
	cacheKey := previewKey(tmdbID, mediaType, region)
	tmdbData, hit := h.previews.get(cacheKey)
	if !hit {
		var err error
		tmdbData, err = h.buildPreviewTMDBData(r.Context(), tmdbID, mediaType, region)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		h.previews.set(cacheKey, tmdbData)
	}

	data := h.baseData("search", user, settings)
	data["Query"] = query
	data["TMDBID"] = tmdbID
	data["MediaType"] = mediaType
	for k, v := range tmdbData {
		data[k] = v
	}

	libMap, _ := h.libraryRepo.GetLibraryMap(r.Context(), userID)
	key := fmt.Sprintf("%d:%s", tmdbID, mediaType)
	if id, ok := libMap[key]; ok {
		data["InLibrary"] = true
		data["MediaID"] = id
	}
	h.render(w, "preview", r, data)
}

// buildPreviewTMDBData fires every TMDB call the preview page needs and
// returns the derived template fields as a map. Does NOT include
// per-user state — that's merged in by Preview on every request so the
// cache stays user-agnostic.
func (h *PageHandler) buildPreviewTMDBData(ctx context.Context, tmdbID int, mediaType, region string) (map[string]any, error) {
	out := map[string]any{}

	if mediaType == "movie" {
		details, err := h.tmdbClient.GetMovie(ctx, tmdbID)
		if err != nil {
			return nil, err
		}
		out["Title"] = details.Title
		out["Overview"] = details.Overview
		out["PosterPath"] = details.PosterPath
		out["ReleaseDate"] = details.ReleaseDate
		out["Runtime"] = details.Runtime
		out["VoteAverage"] = details.VoteAverage
		genres := make([]string, len(details.Genres))
		for i, g := range details.Genres {
			genres[i] = g.Name
		}
		out["Genres"] = strings.Join(genres, ", ")

		if providers, err := h.tmdbClient.GetMovieWatchProviders(ctx, tmdbID, region); err == nil && providers != nil {
			out["Providers"] = providers
		}
		return out, nil
	}

	details, err := h.tmdbClient.GetTV(ctx, tmdbID)
	if err != nil {
		return nil, err
	}
	out["Title"] = details.Name
	out["Overview"] = details.Overview
	out["PosterPath"] = details.PosterPath
	out["ReleaseDate"] = details.FirstAirDate
	out["VoteAverage"] = details.VoteAverage
	genres := make([]string, len(details.Genres))
	for i, g := range details.Genres {
		genres[i] = g.Name
	}
	out["Genres"] = strings.Join(genres, ", ")

	// Per-season episode lists — fetched in parallel (bounded) so
	// even a 26-season show renders in ~one HTTP roundtrip worth
	// of latency. Read-only: the preview page doesn't have watch /
	// rate affordances since the item isn't in the library.
	type previewEpisode struct {
		EpisodeNumber  int
		Name           string
		AirDate        string
		Overview       string
		RuntimeMinutes int
	}
	type previewSeason struct {
		Name         string
		SeasonNumber int
		EpisodeCount int
		AirDate      string
		Episodes     []previewEpisode
	}

	seasonDetails := make([]*tmdb.SeasonDetails, len(details.Seasons))
	var (
		wg  sync.WaitGroup
		sem = make(chan struct{}, 6)
	)
	for i, ts := range details.Seasons {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx, sn int) {
			defer wg.Done()
			defer func() { <-sem }()
			sd, err := h.tmdbClient.GetSeason(ctx, tmdbID, sn)
			if err != nil {
				return
			}
			seasonDetails[idx] = sd
		}(i, ts.SeasonNumber)
	}
	wg.Wait()

	var seasons []previewSeason
	for i, ts := range details.Seasons {
		ps := previewSeason{
			Name:         ts.Name,
			SeasonNumber: ts.SeasonNumber,
			EpisodeCount: ts.EpisodeCount,
			AirDate:      ts.AirDate,
		}
		if seasonDetails[i] != nil {
			for _, e := range seasonDetails[i].Episodes {
				ps.Episodes = append(ps.Episodes, previewEpisode{
					EpisodeNumber:  e.EpisodeNumber,
					Name:           e.Name,
					AirDate:        e.AirDate,
					Overview:       e.Overview,
					RuntimeMinutes: e.Runtime,
				})
			}
		}
		seasons = append(seasons, ps)
	}
	out["PreviewSeasons"] = seasons

	if providers, err := h.tmdbClient.GetTVWatchProviders(ctx, tmdbID, region); err == nil && providers != nil {
		out["Providers"] = providers
	}
	return out, nil
}

func (h *PageHandler) HandleSearchAddAndView(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	tmdbID, _ := strconv.Atoi(r.FormValue("tmdb_id"))
	mediaType := model.MediaType(r.FormValue("media_type"))
	v, err := h.library.AddWithStub(r.Context(), userID, tmdbID, mediaType, readAddStub(r))
	if err != nil {
		existing, _ := h.libraryRepo.GetByTMDBID(r.Context(), userID, tmdbID, mediaType)
		if existing != nil {
			http.Redirect(w, r, "/media/"+existing.Entry.ID, http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/search", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/media/"+v.Entry.ID, http.StatusSeeOther)
}
