package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

type DiscoverHandler struct {
	discovery model.DiscoveryService
	users     model.UserService
}

func NewDiscoverHandler(discovery model.DiscoveryService, users model.UserService) *DiscoverHandler {
	return &DiscoverHandler{discovery: discovery, users: users}
}

func (h *DiscoverHandler) Trending(w http.ResponseWriter, r *http.Request) {
	res, err := h.discovery.Trending(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *DiscoverHandler) Popular(w http.ResponseWriter, r *http.Request) {
	mediaType := model.MediaType(r.URL.Query().Get("type"))
	if mediaType == "" {
		mediaType = model.MediaTypeMovie
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	res, err := h.discovery.Popular(r.Context(), mediaType, page)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *DiscoverHandler) Recommendations(w http.ResponseWriter, r *http.Request) {
	res, err := h.discovery.Recommendations(r.Context(), authUserID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": res})
}

func (h *DiscoverHandler) WatchProviders(w http.ResponseWriter, r *http.Request) {
	libraryID := chi.URLParam(r, "id")
	settings, _ := h.users.GetSettings(r.Context(), authUserID(r))
	region := "NL"
	if settings != nil && settings.Region != "" {
		region = settings.Region
	}
	res, err := h.discovery.WatchProviders(r.Context(), libraryID, region)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *DiscoverHandler) MediaRecommendations(w http.ResponseWriter, r *http.Request) {
	libraryID := chi.URLParam(r, "id")
	res, err := h.discovery.MediaRecommendations(r.Context(), libraryID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": res})
}
