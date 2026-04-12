package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// LibraryImportRunner glues the durable LibraryImportJob queue to the
// LibraryImporter so the page handler and the startup-recovery loop can
// share one entry point.
type LibraryImportRunner struct {
	jobs     model.LibraryImportJobRepository
	importer *LibraryImporter
}

func NewLibraryImportRunner(jobs model.LibraryImportJobRepository, importer *LibraryImporter) *LibraryImportRunner {
	return &LibraryImportRunner{jobs: jobs, importer: importer}
}

// Enqueue persists a freshly-uploaded payload, then starts processing it
// in a background goroutine. Returns the job id immediately so the HTTP
// handler can redirect without blocking on the import.
func (r *LibraryImportRunner) Enqueue(ctx context.Context, userID string, data *LibraryExport) (string, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	job := &model.LibraryImportJob{UserID: userID, Payload: string(payload)}
	if err := r.jobs.Create(ctx, job); err != nil {
		return "", err
	}
	go r.runJob(*job)
	return job.ID, nil
}

// ResumeAll re-runs every job that was left over from a previous server
// run (status pending or running). Called from main.go on startup. Each
// job runs in its own goroutine so a slow TMDB queue for one user can't
// block the others.
func (r *LibraryImportRunner) ResumeAll(ctx context.Context) {
	jobs, err := r.jobs.ListAll(ctx)
	if err != nil {
		slog.Error("library import: list jobs on startup", "error", err)
		return
	}
	for _, j := range jobs {
		if j.Status == "failed" {
			// Don't auto-retry failed jobs — leave them for the user
			// to inspect and re-upload manually.
			continue
		}
		slog.Info("library import: resuming", "job_id", j.ID, "user_id", j.UserID)
		go r.runJob(j)
	}
}

// runJob is the actual processing loop. Always uses context.Background()
// so HTTP cancellation / shutdown timeouts can't kill it mid-run.
func (r *LibraryImportRunner) runJob(j model.LibraryImportJob) {
	ctx := context.Background()
	if err := r.jobs.MarkRunning(ctx, j.ID); err != nil {
		slog.Error("library import: mark running", "job_id", j.ID, "error", err)
		return
	}

	var data LibraryExport
	if err := json.Unmarshal([]byte(j.Payload), &data); err != nil {
		_ = r.jobs.MarkFailed(ctx, j.ID, "decode: "+err.Error())
		slog.Error("library import: decode payload", "job_id", j.ID, "error", err)
		return
	}

	res, err := r.importer.Import(ctx, j.UserID, &data)
	if err != nil {
		_ = r.jobs.MarkFailed(ctx, j.ID, err.Error())
		slog.Error("library import: failed", "job_id", j.ID, "user_id", j.UserID, "error", err)
		return
	}

	slog.Info("library import: complete",
		"job_id", j.ID,
		"user_id", j.UserID,
		"movies_added", res.MoviesAdded, "movies_skipped", res.MoviesSkipped,
		"shows_added", res.ShowsAdded, "shows_skipped", res.ShowsSkipped,
		"errors", len(res.Errors))
	for _, e := range res.Errors {
		slog.Warn("library import: item error", "job_id", j.ID, "error", e)
	}

	if err := r.jobs.Delete(ctx, j.ID); err != nil {
		slog.Warn("library import: cleanup failed", "job_id", j.ID, "error", err)
	}
}
