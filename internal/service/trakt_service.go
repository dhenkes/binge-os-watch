package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// TraktServiceImpl handles importing Trakt JSON exports.
type TraktServiceImpl struct {
	library model.LibraryService
}

func NewTraktService(library model.LibraryService) *TraktServiceImpl {
	return &TraktServiceImpl{library: library}
}

// traktEntry represents a single item in the Trakt JSON export.
type traktEntry struct {
	Type  string `json:"type"`
	Movie *struct {
		IDs struct {
			TMDB int `json:"tmdb"`
		} `json:"ids"`
	} `json:"movie,omitempty"`
	Show *struct {
		IDs struct {
			TMDB int `json:"tmdb"`
		} `json:"ids"`
	} `json:"show,omitempty"`
}

// Import parses a Trakt JSON export, adds items to the user's library, and
// marks movies as watched. Returns (added, skipped, error).
func (s *TraktServiceImpl) Import(ctx context.Context, userID string, data []byte) (int, int, error) {
	var entries []traktEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return 0, 0, fmt.Errorf("parsing trakt export: %w", err)
	}

	var added, skipped int
	for _, e := range entries {
		var tmdbID int
		var mediaType model.MediaType

		switch e.Type {
		case "movie":
			if e.Movie == nil || e.Movie.IDs.TMDB == 0 {
				skipped++
				continue
			}
			tmdbID = e.Movie.IDs.TMDB
			mediaType = model.MediaTypeMovie
		case "episode", "show":
			if e.Show == nil || e.Show.IDs.TMDB == 0 {
				skipped++
				continue
			}
			tmdbID = e.Show.IDs.TMDB
			mediaType = model.MediaTypeTV
		default:
			skipped++
			continue
		}

		v, err := s.library.Add(ctx, userID, tmdbID, mediaType)
		if err != nil {
			var appErr *model.AppError
			if errors.As(err, &appErr) && appErr.Code == model.ErrorCodeAlreadyExists {
				skipped++
				continue
			}
			skipped++
			continue
		}

		if mediaType == model.MediaTypeMovie {
			watched := model.MediaStatusWatched
			_ = s.library.SetStatus(ctx, v.Entry.ID, &watched)
		}
		added++
	}

	return added, skipped, nil
}
