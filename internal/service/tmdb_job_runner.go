package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// TMDBJobRunner is the durable worker for all background TMDB fetches.
// The UI never blocks on TMDB — handlers enqueue jobs here and return
// immediately. The runner resumes in-flight jobs on startup so a crash
// mid-fetch doesn't lose work.
//
// The runner dispatches by TMDBJob.Kind — right now that's
// add_catalog (called by LibraryService.Add) and refresh_catalog
// (called by MetadataSync via LibraryService.RefreshCatalog). New kinds
// are added by adding a case to runJob and a payload struct below.
type TMDBJobRunner struct {
	jobs    model.TMDBJobRepository
	library *LibraryServiceImpl
}

func NewTMDBJobRunner(jobs model.TMDBJobRepository, library *LibraryServiceImpl) *TMDBJobRunner {
	return &TMDBJobRunner{jobs: jobs, library: library}
}

// AddCatalogPayload is the shape of a TMDBJobAddCatalog payload: the
// tmdb_show.id or tmdb_movie.id the catalog row to refresh, plus the
// TMDB media type. The placeholder row at catalog_id already exists — the
// runner just needs to flesh it out.
type AddCatalogPayload struct {
	TMDBID    int    `json:"tmdb_id"`
	MediaType string `json:"media_type"`
}

// RefreshCatalogPayload is the shape of a TMDBJobRefreshCatalog payload.
// Right now it's identical to AddCatalogPayload — kept as a distinct
// type so the two job kinds can evolve independently.
type RefreshCatalogPayload struct {
	TMDBID    int    `json:"tmdb_id"`
	MediaType string `json:"media_type"`
}

// EnqueueAddCatalog persists a pending add_catalog job and kicks the
// worker goroutine. Returns the job id for callers that want to poll
// status (tests, potential UI).
func (r *TMDBJobRunner) EnqueueAddCatalog(ctx context.Context, userID string, tmdbID int, mediaType string) (string, error) {
	pl, _ := json.Marshal(AddCatalogPayload{TMDBID: tmdbID, MediaType: mediaType})
	uid := userID
	job := &model.TMDBJob{
		UserID:  &uid,
		Kind:    string(model.TMDBJobAddCatalog),
		Payload: string(pl),
	}
	if err := r.jobs.Create(ctx, job); err != nil {
		return "", err
	}
	go r.runJob(*job)
	return job.ID, nil
}

// EnqueueRefreshCatalog is the same but for the refresh-all flow.
func (r *TMDBJobRunner) EnqueueRefreshCatalog(ctx context.Context, tmdbID int, mediaType string) (string, error) {
	pl, _ := json.Marshal(RefreshCatalogPayload{TMDBID: tmdbID, MediaType: mediaType})
	job := &model.TMDBJob{
		Kind:    string(model.TMDBJobRefreshCatalog),
		Payload: string(pl),
	}
	if err := r.jobs.Create(ctx, job); err != nil {
		return "", err
	}
	go r.runJob(*job)
	return job.ID, nil
}

// ResumeAll re-kicks every tmdb_job on startup — including failed ones.
// The reasoning: a tmdb_job almost always fails because TMDB was flaky
// at the time. Retrying on boot (and again via RunRetryLoop) is the
// cheap way to heal "stuck Loading…" library rows without admin action.
// Each job gets its own goroutine so a stuck TMDB call can't block the
// others.
func (r *TMDBJobRunner) ResumeAll(ctx context.Context) {
	jobs, err := r.jobs.ListAll(ctx)
	if err != nil {
		slog.Error("tmdb job: list on startup", "error", err)
		return
	}
	for _, j := range jobs {
		slog.Info("tmdb job: resuming", "job_id", j.ID, "kind", j.Kind, "status", j.Status)
		go r.runJob(j)
	}
}

// RunRetryLoop periodically re-kicks any failed tmdb_jobs so a stuck
// "Loading…" row from an earlier TMDB outage heals on its own. Blocks
// until ctx is cancelled. Intended to be run in a goroutine from main.go
// alongside the other background jobs.
func (r *TMDBJobRunner) RunRetryLoop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	slog.Info("tmdb job retry loop started", "interval", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.retryFailed(ctx)
		case <-ctx.Done():
			slog.Info("tmdb job retry loop stopped")
			return
		}
	}
}

func (r *TMDBJobRunner) retryFailed(ctx context.Context) {
	jobs, err := r.jobs.ListAll(ctx)
	if err != nil {
		slog.Warn("tmdb job retry: list", "error", err)
		return
	}
	n := 0
	for _, j := range jobs {
		if j.Status != "failed" {
			continue
		}
		slog.Info("tmdb job retry: re-running", "job_id", j.ID, "kind", j.Kind, "prev_error", j.Error)
		go r.runJob(j)
		n++
	}
	if n > 0 {
		slog.Info("tmdb job retry: kicked", "count", n)
	}
}

// RetryOne kicks a single job in a goroutine without touching any
// queue-scanning logic. Used by the admin "retry" button.
func (r *TMDBJobRunner) RetryOne(j model.TMDBJob) {
	go r.runJob(j)
}

// runJob is the actual processing loop. Always uses context.Background()
// so HTTP cancellation / shutdown timeouts can't kill it mid-run.
func (r *TMDBJobRunner) runJob(j model.TMDBJob) {
	ctx := context.Background()
	if err := r.jobs.MarkRunning(ctx, j.ID); err != nil {
		slog.Error("tmdb job: mark running", "job_id", j.ID, "error", err)
		return
	}

	var err error
	switch model.TMDBJobKind(j.Kind) {
	case model.TMDBJobAddCatalog:
		var pl AddCatalogPayload
		if e := json.Unmarshal([]byte(j.Payload), &pl); e != nil {
			err = fmt.Errorf("decode add_catalog payload: %w", e)
			break
		}
		err = r.library.CompleteCatalogFetch(ctx, pl.TMDBID, pl.MediaType)

	case model.TMDBJobRefreshCatalog:
		var pl RefreshCatalogPayload
		if e := json.Unmarshal([]byte(j.Payload), &pl); e != nil {
			err = fmt.Errorf("decode refresh_catalog payload: %w", e)
			break
		}
		err = r.library.CompleteCatalogFetch(ctx, pl.TMDBID, pl.MediaType)

	default:
		err = fmt.Errorf("unknown tmdb job kind: %q", j.Kind)
	}

	if err != nil {
		_ = r.jobs.MarkFailed(ctx, j.ID, err.Error())
		slog.Error("tmdb job: failed", "job_id", j.ID, "kind", j.Kind, "error", err)
		return
	}

	slog.Info("tmdb job: complete", "job_id", j.ID, "kind", j.Kind)
	if err := r.jobs.Delete(ctx, j.ID); err != nil {
		slog.Warn("tmdb job: cleanup", "job_id", j.ID, "error", err)
	}
}
