package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// MediaHandler exposes /api/v1/media endpoints. Even though the underlying
// schema split the old `media` table into `tmdb_show`/`tmdb_movie` plus
// `user_library`, the API still calls these "media" for backwards
// compatibility — each "media id" in the JSON contract maps to a
// user_library row id.
type MediaHandler struct {
	library     model.LibraryService
	libraryRepo model.LibraryRepository
}

func NewMediaHandler(library model.LibraryService, libraryRepo model.LibraryRepository) *MediaHandler {
	return &MediaHandler{library: library, libraryRepo: libraryRepo}
}

func (h *MediaHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := model.LibraryFilter{
		Status:    model.MediaStatus(q.Get("status")),
		MediaType: model.MediaType(q.Get("type")),
		TagID:     q.Get("tag"),
		Query:     q.Get("q"),
		SortBy:    q.Get("sort"),
		SortDir:   q.Get("dir"),
	}
	page := pageRequest(r)
	res, err := h.libraryRepo.List(r.Context(), authUserID(r), filter, page)
	if err != nil {
		writeError(w, err)
		return
	}
	writePageJSON(w, res)
}

type addMediaRequest struct {
	TMDBID    int             `json:"tmdb_id"`
	MediaType model.MediaType `json:"media_type"`
}

func (h *MediaHandler) Add(w http.ResponseWriter, r *http.Request) {
	var req addMediaRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	v, err := h.library.Add(r.Context(), authUserID(r), req.TMDBID, req.MediaType)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (h *MediaHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	v, err := h.libraryRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

type setStatusRequest struct {
	Status model.MediaStatus `json:"status"`
}

func (h *MediaHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
	var req setStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	var status *model.MediaStatus
	if req.Status != "" {
		status = &req.Status
	}
	if err := h.library.SetStatus(r.Context(), id, status); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *MediaHandler) Remove(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.library.Remove(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
