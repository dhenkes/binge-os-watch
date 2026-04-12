package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

type RatingHandler struct {
	ratings     model.RatingServiceV2
	libraryRepo model.LibraryRepository
	episodeRepo model.TMDBEpisodeRepository
	seasonRepo  model.TMDBSeasonRepository
}

func NewRatingHandler(
	ratings model.RatingServiceV2,
	libraryRepo model.LibraryRepository,
	episodeRepo model.TMDBEpisodeRepository,
	seasonRepo model.TMDBSeasonRepository,
) *RatingHandler {
	return &RatingHandler{
		ratings:     ratings,
		libraryRepo: libraryRepo,
		episodeRepo: episodeRepo,
		seasonRepo:  seasonRepo,
	}
}

type rateRequest struct {
	Score int `json:"score"`
}

// RateMedia rates a library entry — resolves to either rate_show or
// rate_movie depending on the entry's media type.
func (h *RatingHandler) RateMedia(w http.ResponseWriter, r *http.Request) {
	var req rateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	v, err := h.libraryRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	switch {
	case v.Show != nil:
		if err := h.ratings.RateShow(r.Context(), authUserID(r), v.Show.ID, req.Score); err != nil {
			writeError(w, err)
			return
		}
	case v.Movie != nil:
		if err := h.ratings.RateMovie(r.Context(), authUserID(r), v.Movie.ID, req.Score); err != nil {
			writeError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RatingHandler) RateSeason(w http.ResponseWriter, r *http.Request) {
	var req rateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.ratings.RateSeason(r.Context(), authUserID(r), id, req.Score); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RatingHandler) RateEpisode(w http.ResponseWriter, r *http.Request) {
	var req rateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.ratings.RateEpisode(r.Context(), authUserID(r), id, req.Score); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteRating is a no-op under the new schema — clients should send
// score=0 to the appropriate Rate* endpoint instead. Kept for OpenAPI
// compatibility, returns 204.
func (h *RatingHandler) DeleteRating(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
