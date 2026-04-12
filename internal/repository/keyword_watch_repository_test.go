package repository

import (
	"context"
	"testing"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

func TestKeywordWatchRepository_CreateAndList(t *testing.T) {
	db := testDB(t)
	userID := seedUserForMedia(t, db)
	repo := NewKeywordWatchRepository(db)
	ctx := context.Background()

	kw := &model.KeywordWatch{UserID: userID, Keyword: "baki", MediaTypes: "movie,tv"}
	if err := repo.Create(ctx, kw); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if kw.ID == "" {
		t.Fatal("ID should be set")
	}

	list, err := repo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser() error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d watches, want 1", len(list))
	}
	if list[0].Keyword != "baki" {
		t.Errorf("Keyword = %q, want baki", list[0].Keyword)
	}
}

func TestKeywordWatchRepository_DuplicateKeyword(t *testing.T) {
	db := testDB(t)
	userID := seedUserForMedia(t, db)
	repo := NewKeywordWatchRepository(db)
	ctx := context.Background()

	repo.Create(ctx, &model.KeywordWatch{UserID: userID, Keyword: "dup", MediaTypes: "movie"})
	err := repo.Create(ctx, &model.KeywordWatch{UserID: userID, Keyword: "dup", MediaTypes: "tv"})
	if err == nil {
		t.Fatal("expected already exists error")
	}
}

func TestKeywordWatchRepository_GetByID(t *testing.T) {
	db := testDB(t)
	userID := seedUserForMedia(t, db)
	repo := NewKeywordWatchRepository(db)
	ctx := context.Background()

	kw := &model.KeywordWatch{UserID: userID, Keyword: "test", MediaTypes: "movie"}
	repo.Create(ctx, kw)

	got, err := repo.GetByID(ctx, kw.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.Keyword != "test" {
		t.Errorf("Keyword = %q, want test", got.Keyword)
	}
}

func TestKeywordWatchRepository_Update(t *testing.T) {
	db := testDB(t)
	userID := seedUserForMedia(t, db)
	repo := NewKeywordWatchRepository(db)
	ctx := context.Background()

	kw := &model.KeywordWatch{UserID: userID, Keyword: "old", MediaTypes: "movie"}
	repo.Create(ctx, kw)

	kw.Keyword = "new"
	if err := repo.Update(ctx, kw, []string{"keyword"}); err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	got, _ := repo.GetByID(ctx, kw.ID)
	if got.Keyword != "new" {
		t.Errorf("Keyword = %q, want new", got.Keyword)
	}
}

func TestKeywordWatchRepository_Delete(t *testing.T) {
	db := testDB(t)
	userID := seedUserForMedia(t, db)
	repo := NewKeywordWatchRepository(db)
	ctx := context.Background()

	kw := &model.KeywordWatch{UserID: userID, Keyword: "del", MediaTypes: "movie"}
	repo.Create(ctx, kw)
	repo.Delete(ctx, kw.ID)

	_, err := repo.GetByID(ctx, kw.ID)
	if err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestKeywordWatchRepository_Results(t *testing.T) {
	db := testDB(t)
	userID := seedUserForMedia(t, db)
	repo := NewKeywordWatchRepository(db)
	ctx := context.Background()

	kw := &model.KeywordWatch{UserID: userID, Keyword: "baki", MediaTypes: "movie"}
	repo.Create(ctx, kw)

	// Create result.
	result := &model.KeywordResult{
		KeywordWatchID: kw.ID,
		TMDBID:         12345,
		MediaType:      model.MediaTypeMovie,
		Title:          "Baki Movie",
		Status:         model.KeywordResultPending,
	}
	if err := repo.CreateResult(ctx, result); err != nil {
		t.Fatalf("CreateResult() error: %v", err)
	}

	// List pending.
	pending, err := repo.ListPendingResults(ctx, userID)
	if err != nil {
		t.Fatalf("ListPendingResults() error: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("got %d pending, want 1", len(pending))
	}

	// Pending count.
	count, _ := repo.PendingCount(ctx, userID)
	if count != 1 {
		t.Errorf("PendingCount = %d, want 1", count)
	}

	// Result exists.
	exists, _ := repo.ResultExists(ctx, kw.ID, 12345, model.MediaTypeMovie)
	if !exists {
		t.Error("ResultExists should be true")
	}
	exists, _ = repo.ResultExists(ctx, kw.ID, 99999, model.MediaTypeMovie)
	if exists {
		t.Error("ResultExists should be false for unknown tmdb_id")
	}

	// Dismiss.
	repo.UpdateResultStatus(ctx, result.ID, model.KeywordResultDismissed)
	pending, _ = repo.ListPendingResults(ctx, userID)
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after dismiss, got %d", len(pending))
	}
}

func TestKeywordWatchRepository_DismissAll(t *testing.T) {
	db := testDB(t)
	userID := seedUserForMedia(t, db)
	repo := NewKeywordWatchRepository(db)
	ctx := context.Background()

	kw := &model.KeywordWatch{UserID: userID, Keyword: "test", MediaTypes: "movie"}
	repo.Create(ctx, kw)

	repo.CreateResult(ctx, &model.KeywordResult{KeywordWatchID: kw.ID, TMDBID: 1, MediaType: model.MediaTypeMovie, Title: "A", Status: model.KeywordResultPending})
	repo.CreateResult(ctx, &model.KeywordResult{KeywordWatchID: kw.ID, TMDBID: 2, MediaType: model.MediaTypeMovie, Title: "B", Status: model.KeywordResultPending})

	if err := repo.DismissAllResults(ctx, kw.ID); err != nil {
		t.Fatalf("DismissAllResults() error: %v", err)
	}

	count, _ := repo.PendingCount(ctx, userID)
	if count != 0 {
		t.Errorf("PendingCount = %d, want 0 after dismiss all", count)
	}
}

func TestKeywordWatchRepository_ListAll(t *testing.T) {
	db := testDB(t)
	userID := seedUserForMedia(t, db)
	repo := NewKeywordWatchRepository(db)
	ctx := context.Background()

	repo.Create(ctx, &model.KeywordWatch{UserID: userID, Keyword: "a", MediaTypes: "movie"})
	repo.Create(ctx, &model.KeywordWatch{UserID: userID, Keyword: "b", MediaTypes: "tv"})

	all, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll() error: %v", err)
	}
	if len(all) < 2 {
		t.Errorf("got %d watches, want at least 2", len(all))
	}
}

func TestKeywordWatchRepository_CreateResult_Idempotent(t *testing.T) {
	db := testDB(t)
	userID := seedUserForMedia(t, db)
	repo := NewKeywordWatchRepository(db)
	ctx := context.Background()

	kw := &model.KeywordWatch{UserID: userID, Keyword: "idem", MediaTypes: "movie"}
	repo.Create(ctx, kw)

	r1 := &model.KeywordResult{KeywordWatchID: kw.ID, TMDBID: 1, MediaType: model.MediaTypeMovie, Title: "A", Status: model.KeywordResultPending}
	repo.CreateResult(ctx, r1)

	// Second create for same tmdb_id should be a no-op.
	r2 := &model.KeywordResult{KeywordWatchID: kw.ID, TMDBID: 1, MediaType: model.MediaTypeMovie, Title: "A v2", Status: model.KeywordResultPending}
	if err := repo.CreateResult(ctx, r2); err != nil {
		t.Fatalf("second CreateResult should not error: %v", err)
	}

	count, _ := repo.PendingCount(ctx, userID)
	if count != 1 {
		t.Errorf("PendingCount = %d, want 1 (idempotent)", count)
	}
}
