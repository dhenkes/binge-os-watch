package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

type ImageHandler struct {
	images model.ImageService
}

func NewImageHandler(images model.ImageService) *ImageHandler {
	return &ImageHandler{images: images}
}

// Get handles GET /api/v1/images/*
// The path after /api/v1/images/ is the TMDB image path (e.g. "w342/abc.jpg").
func (h *ImageHandler) Get(w http.ResponseWriter, r *http.Request) {
	path := chi.URLParam(r, "*")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	img, err := h.images.Get(r.Context(), path)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", img.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(img.Data)
}
