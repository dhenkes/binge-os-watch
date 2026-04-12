package model

import "context"

// TMDBJobKind enumerates the different background TMDB jobs that the
// runner can dispatch on. Unknown kinds are logged and failed.
type TMDBJobKind string

const (
	// TMDBJobAddCatalog is enqueued when the user adds a new library
	// entry. Payload is an AddCatalogPayload — the runner fetches the
	// full TMDB catalog for the show/movie (including per-season episode
	// lists) and upserts it in one tx. The user_library row already
	// exists by the time this job runs.
	TMDBJobAddCatalog TMDBJobKind = "add_catalog"

	// TMDBJobRefreshCatalog re-fetches a show's full catalog. Payload is
	// a RefreshCatalogPayload. Used by the metadata sync job for
	// per-show refreshes.
	TMDBJobRefreshCatalog TMDBJobKind = "refresh_catalog"
)

// TMDBJob is the durable queue row. One row = one pending TMDB fetch.
type TMDBJob struct {
	ID        string
	UserID    *string // nullable — some jobs (cache warmers) aren't user-scoped
	Kind      string  // a TMDBJobKind
	Payload   string  // JSON blob the runner decodes based on Kind
	Status    string  // "pending" | "running" | "failed"
	Error     string
	CreatedAt int64
	StartedAt *int64
}

type TMDBJobRepository interface {
	Create(ctx context.Context, job *TMDBJob) error
	MarkRunning(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id, errMsg string) error
	Delete(ctx context.Context, id string) error
	ListAll(ctx context.Context) ([]TMDBJob, error)
}
