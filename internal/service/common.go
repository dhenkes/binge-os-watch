package service

import (
	"context"
	"strings"

	"github.com/dhenkes/binge-os-watch/internal/tmdb"
)

// WebhookDispatcher is the subset of the webhook service that
// non-webhook services depend on. Living here keeps the import graph
// acyclic — library/watch/media services can dispatch events without
// pulling in the concrete webhook implementation.
type WebhookDispatcher interface {
	Dispatch(ctx context.Context, userID, event string, payload any)
}

// genreNames joins TMDB genre names into the comma-separated string we
// persist in the catalog tables.
func genreNames(genres []tmdb.Genre) string {
	names := make([]string, len(genres))
	for i, g := range genres {
		names[i] = g.Name
	}
	return strings.Join(names, ",")
}
