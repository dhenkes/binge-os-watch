package model

import "context"

// LibraryImportJob is a durable record of an in-flight library import.
// The payload column holds the raw validated JSON so a crash mid-import
// (or a server restart) can pick the work back up.
type LibraryImportJob struct {
	ID        string
	UserID    string
	Payload   string // JSON blob
	Status    string // "pending" | "running" | "failed"
	Error     string
	CreatedAt int64
	StartedAt *int64
}

type LibraryImportJobRepository interface {
	Create(ctx context.Context, job *LibraryImportJob) error
	MarkRunning(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id, errMsg string) error
	Delete(ctx context.Context, id string) error
	ListAll(ctx context.Context) ([]LibraryImportJob, error)
}
