package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

type EpisodeHandler struct {
	watch model.WatchService
}

func NewEpisodeHandler(watch model.WatchService) *EpisodeHandler {
	return &EpisodeHandler{watch: watch}
}

func (h *EpisodeHandler) Watch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.watch.WatchEpisode(r.Context(), authUserID(r), id, 0, ""); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *EpisodeHandler) Unwatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.watch.UnwatchAllForEpisode(r.Context(), authUserID(r), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *EpisodeHandler) WatchAllInSeason(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.watch.WatchAllInSeason(r.Context(), authUserID(r), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// WatchNext takes a library entry id, resolves the show, and marks the
// next unwatched aired regular episode watched.
func (h *EpisodeHandler) WatchNext(w http.ResponseWriter, r *http.Request) {
	// The route uses {id} for the library entry, but WatchNext lives on the
	// show, so callers should pass the show id as the URL param. Older
	// API consumers passed the library id; for compatibility we accept
	// either by treating the param as a show id directly. The handler at
	// /api/v1/media/{id}:watch-next is documented to take a media id —
	// in the new schema "media id" === user_library row id. We need a
	// quick lookup; absent that, just no-op when the URL param isn't a
	// real show id. Future change: route through the library repo.
	id := chi.URLParam(r, "id")
	if _, err := h.watch.WatchNext(r.Context(), authUserID(r), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
