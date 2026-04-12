package service

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/dhenkes/binge-os-watch/internal/repository"
	"github.com/google/uuid"
)

// stubEnqueuer records calls made to EnqueueAddCatalog /
// EnqueueRefreshCatalog so tests can assert the async path was taken
// without needing a real TMDBJobRunner.
type stubEnqueuer struct {
	mu       sync.Mutex
	addCalls []stubEnqueueCall
	refCalls []stubEnqueueCall
}

type stubEnqueueCall struct {
	UserID    string
	TMDBID    int
	MediaType string
}

func (s *stubEnqueuer) EnqueueAddCatalog(ctx context.Context, userID string, tmdbID int, mediaType string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addCalls = append(s.addCalls, stubEnqueueCall{userID, tmdbID, mediaType})
	return "stub-job-id", nil
}

func (s *stubEnqueuer) EnqueueRefreshCatalog(ctx context.Context, tmdbID int, mediaType string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refCalls = append(s.refCalls, stubEnqueueCall{"", tmdbID, mediaType})
	return "stub-job-id", nil
}

// TestLibraryService_AddWithStub_Movie covers the happy path for the
// async movie add:
//   - a placeholder tmdb_movie row is created with the stub title and
//     RefreshedAt=0 (the "still loading" marker the templates use)
//   - the user_library row is created immediately and points at the
//     placeholder movie row
//   - exactly one add_catalog job is enqueued with the right args
//   - AddWithStub returns without waiting for the background fetch
func TestLibraryService_AddWithStub_Movie(t *testing.T) {
	svc, stub, userID := newAsyncTestService(t)
	ctx := context.Background()

	stubData := &model.AddStub{
		Title:       "The Matrix",
		Overview:    "Welcome to the real world.",
		PosterPath:  "/matrix.jpg",
		ReleaseDate: "1999-03-31",
	}
	view, err := svc.AddWithStub(ctx, userID, 603, model.MediaTypeMovie, stubData)
	if err != nil {
		t.Fatalf("AddWithStub: %v", err)
	}

	if view == nil || view.Movie == nil {
		t.Fatalf("expected library view with movie, got %+v", view)
	}
	if view.Movie.Title != "The Matrix" {
		t.Errorf("placeholder title = %q, want The Matrix", view.Movie.Title)
	}
	if view.Movie.RefreshedAt != 0 {
		t.Errorf("placeholder RefreshedAt = %d, want 0 (placeholder marker)", view.Movie.RefreshedAt)
	}
	if view.Movie.TMDBID != 603 {
		t.Errorf("placeholder TMDBID = %d, want 603", view.Movie.TMDBID)
	}
	if view.Entry.MovieID == nil || *view.Entry.MovieID != view.Movie.ID {
		t.Errorf("library entry.MovieID does not point at placeholder")
	}

	if len(stub.addCalls) != 1 {
		t.Fatalf("want 1 EnqueueAddCatalog call, got %d", len(stub.addCalls))
	}
	got := stub.addCalls[0]
	if got.UserID != userID || got.TMDBID != 603 || got.MediaType != "movie" {
		t.Errorf("enqueue call = %+v, want {user, 603, movie}", got)
	}
}

// TestLibraryService_AddWithStub_TV covers the TV flavor — placeholder
// tmdb_show row, no seasons/episodes yet, job enqueued.
func TestLibraryService_AddWithStub_TV(t *testing.T) {
	svc, stub, userID := newAsyncTestService(t)
	ctx := context.Background()

	view, err := svc.AddWithStub(ctx, userID, 1399, model.MediaTypeTV, &model.AddStub{
		Title:      "Game of Thrones",
		PosterPath: "/got.jpg",
	})
	if err != nil {
		t.Fatalf("AddWithStub: %v", err)
	}
	if view == nil || view.Show == nil {
		t.Fatalf("expected library view with show")
	}
	if view.Show.Title != "Game of Thrones" {
		t.Errorf("show title = %q, want Game of Thrones", view.Show.Title)
	}
	if view.Show.RefreshedAt != 0 {
		t.Errorf("RefreshedAt = %d, want 0", view.Show.RefreshedAt)
	}
	if len(stub.addCalls) != 1 {
		t.Fatalf("want 1 enqueue call, got %d", len(stub.addCalls))
	}
	if stub.addCalls[0].MediaType != "tv" {
		t.Errorf("enqueued media type = %q, want tv", stub.addCalls[0].MediaType)
	}
}

// TestLibraryService_AddWithStub_NilStub verifies the no-stub path:
// without a stub the placeholder falls back to "Loading…" so the
// library list still has something readable to show.
func TestLibraryService_AddWithStub_NilStub(t *testing.T) {
	svc, _, userID := newAsyncTestService(t)
	view, err := svc.AddWithStub(context.Background(), userID, 42, model.MediaTypeMovie, nil)
	if err != nil {
		t.Fatalf("AddWithStub: %v", err)
	}
	if view.Movie.Title != "Loading…" {
		t.Errorf("nil-stub placeholder title = %q, want Loading…", view.Movie.Title)
	}
}

// TestLibraryService_AddWithStub_Duplicate checks the duplicate guard
// still runs before any enqueue happens.
func TestLibraryService_AddWithStub_Duplicate(t *testing.T) {
	svc, stub, userID := newAsyncTestService(t)
	ctx := context.Background()

	_, err := svc.AddWithStub(ctx, userID, 603, model.MediaTypeMovie, &model.AddStub{Title: "A"})
	if err != nil {
		t.Fatalf("first add: %v", err)
	}

	_, err = svc.AddWithStub(ctx, userID, 603, model.MediaTypeMovie, &model.AddStub{Title: "A"})
	appErr, _ := err.(*model.AppError)
	if appErr == nil || appErr.Code != model.ErrorCodeAlreadyExists {
		t.Errorf("second add err = %v, want AlreadyExists", err)
	}
	if len(stub.addCalls) != 1 {
		t.Errorf("duplicate should not enqueue a second job — got %d calls", len(stub.addCalls))
	}
}

// newAsyncTestService builds a LibraryServiceImpl backed by real repos
// on an in-memory DB, with a stub catalog enqueuer wired in so the
// async path is exercised without needing a live TMDB client.
func newAsyncTestService(t *testing.T) (*LibraryServiceImpl, *stubEnqueuer, string) {
	t.Helper()
	dsn := fmt.Sprintf("file:async_test_%p?mode=memory&cache=shared", t)
	db, err := repository.NewSQLiteDB(dsn)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()

	userID := uuid.NewString()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO users (id, username, password, role, created_at)
		 VALUES (?, ?, ?, 'user', strftime('%s','now'))`,
		userID, "asyncer", "hash"); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	txFunc := repository.NewTxFunc(db)
	svc := NewLibraryService(
		txFunc,
		repository.NewTMDBShowRepository(db),
		repository.NewTMDBMovieRepository(db),
		repository.NewTMDBSeasonRepository(db),
		repository.NewTMDBEpisodeRepository(db),
		repository.NewLibraryRepository(db),
		nil, // *tmdb.Client — unused on the sync path we're testing
	)
	stub := &stubEnqueuer{}
	svc.SetTMDBJobRunner(stub)
	return svc, stub, userID
}
