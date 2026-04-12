package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

type TagHandler struct {
	tags       model.TagService
	libraryTag model.LibraryTagRepository
}

func NewTagHandler(tags model.TagService, libraryTag model.LibraryTagRepository) *TagHandler {
	return &TagHandler{tags: tags, libraryTag: libraryTag}
}

func (h *TagHandler) List(w http.ResponseWriter, r *http.Request) {
	res, err := h.tags.List(r.Context(), authUserID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": res})
}

type createTagRequest struct {
	Name string `json:"name"`
}

func (h *TagHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createTagRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	tag, err := h.tags.Create(r.Context(), authUserID(r), req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tag)
}

func (h *TagHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.tags.Delete(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type addTagRequest struct {
	TagID string `json:"tag_id"`
}

// AddToMedia attaches an existing tag to a library entry.
func (h *TagHandler) AddToMedia(w http.ResponseWriter, r *http.Request) {
	var req addTagRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	libraryID := chi.URLParam(r, "id")
	if err := h.libraryTag.Add(r.Context(), libraryID, req.TagID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TagHandler) RemoveFromMedia(w http.ResponseWriter, r *http.Request) {
	libraryID := chi.URLParam(r, "id")
	tagID := chi.URLParam(r, "tagId")
	if err := h.libraryTag.Remove(r.Context(), libraryID, tagID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
