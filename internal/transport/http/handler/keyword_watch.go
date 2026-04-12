package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

type KeywordWatchHandler struct {
	keywords model.KeywordWatchService
}

func NewKeywordWatchHandler(keywords model.KeywordWatchService) *KeywordWatchHandler {
	return &KeywordWatchHandler{keywords: keywords}
}

func (h *KeywordWatchHandler) List(w http.ResponseWriter, r *http.Request) {
	res, err := h.keywords.List(r.Context(), authUserID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": res})
}

type createKeywordRequest struct {
	Keyword    string `json:"keyword"`
	MediaTypes string `json:"media_types"`
}

func (h *KeywordWatchHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createKeywordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	kw, err := h.keywords.Create(r.Context(), authUserID(r), req.Keyword, req.MediaTypes)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, kw)
}

func (h *KeywordWatchHandler) Update(w http.ResponseWriter, r *http.Request) {
	var kw model.KeywordWatch
	if err := decodeJSON(r, &kw); err != nil {
		writeError(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.keywords.Update(r.Context(), id, updateMask(r), &kw); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, kw)
}

func (h *KeywordWatchHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.keywords.Delete(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *KeywordWatchHandler) Suggestions(w http.ResponseWriter, r *http.Request) {
	res, err := h.keywords.ListSuggestions(r.Context(), authUserID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": res})
}

func (h *KeywordWatchHandler) PendingCount(w http.ResponseWriter, r *http.Request) {
	n, err := h.keywords.PendingCount(r.Context(), authUserID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"count": n})
}

func (h *KeywordWatchHandler) DismissAll(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.keywords.DismissAll(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *KeywordWatchHandler) AddResult(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.keywords.AddSuggestion(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *KeywordWatchHandler) DismissResult(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.keywords.DismissSuggestion(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
