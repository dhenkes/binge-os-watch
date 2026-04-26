package handler

import (
	"net/http"
	"strconv"

	"github.com/dhenkes/binge-os-watch/internal/tmdb"
)

type SearchHandler struct {
	tmdb *tmdb.Client
}

func NewSearchHandler(client *tmdb.Client) *SearchHandler {
	return &SearchHandler{tmdb: client}
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"results": []any{}})
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	var (
		resp *tmdb.SearchResponse
		err  error
	)
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	if year < 0 {
		year = 0
	}
	switch r.URL.Query().Get("type") {
	case "movie":
		resp, err = h.tmdb.SearchMovies(r.Context(), q, page, year)
	case "tv":
		resp, err = h.tmdb.SearchTV(r.Context(), q, page, year)
	default:
		resp, err = h.tmdb.SearchMulti(r.Context(), q, page)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
