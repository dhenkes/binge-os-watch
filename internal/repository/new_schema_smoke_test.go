package repository

import (
	"context"
	"testing"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/google/uuid"
)

// TestNewSchema_EndToEndSmoke drives a complete add-show → mark-watched →
// list path against the draft Option B schema. It doesn't exhaustively
// cover every method, but it proves the wiring of every new repository
// against the real SQL before Phase C plumbs them into services.
func TestNewSchema_EndToEndSmoke(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// Seed a user — users table is unchanged so we can insert directly.
	userID := uuid.NewString()
	_, err := db.ExecContext(ctx,
		`INSERT INTO users (id, username, password, role, created_at)
		 VALUES (?, ?, ?, 'user', strftime('%s','now'))`,
		userID, "alice", "hash")
	if err != nil {
		t.Fatalf("seeding user: %v", err)
	}

	shows := NewTMDBShowRepository(db)
	seasons := NewTMDBSeasonRepository(db)
	episodes := NewTMDBEpisodeRepository(db)
	library := NewLibraryRepository(db)
	events := NewWatchEventRepository(db)
	ratings := NewRatingV2Repository(db)
	tags := NewLibraryTagRepository(db)

	// --- Catalog: one show, one season, three episodes (one aired, one
	// future, one special).
	show := &model.TMDBShow{TMDBID: 1399, Title: "GOT", RefreshedAt: 1}
	if err := shows.Upsert(ctx, show); err != nil {
		t.Fatalf("upsert show: %v", err)
	}
	regularSeason := &model.TMDBSeason{ShowID: show.ID, SeasonNumber: 1, Name: "S1", TMDBSeasonID: 10}
	specialsSeason := &model.TMDBSeason{ShowID: show.ID, SeasonNumber: 0, Name: "Specials", TMDBSeasonID: 0}
	if err := seasons.UpsertBatch(ctx, []model.TMDBSeason{*regularSeason, *specialsSeason}); err != nil {
		t.Fatalf("upsert seasons: %v", err)
	}
	// seasons.UpsertBatch filled in IDs on the slice we passed in, but
	// our local pointers still have empty IDs; re-fetch.
	allSeasons, _ := seasons.ListByShow(ctx, show.ID)
	var regularID, specialsID string
	for _, s := range allSeasons {
		if s.SeasonNumber == 1 {
			regularID = s.ID
		} else {
			specialsID = s.ID
		}
	}

	pastAir := int64(1)
	futureAir := int64(9999999999)
	eps := []model.TMDBEpisode{
		{SeasonID: regularID, TMDBEpisodeID: 1, EpisodeNumber: 1, Name: "Aired", AirDate: &pastAir},
		{SeasonID: regularID, TMDBEpisodeID: 2, EpisodeNumber: 2, Name: "Future", AirDate: &futureAir},
		{SeasonID: specialsID, TMDBEpisodeID: 100, EpisodeNumber: 1, Name: "Special", AirDate: &pastAir},
	}
	if err := episodes.UpsertBatch(ctx, eps); err != nil {
		t.Fatalf("upsert episodes: %v", err)
	}
	allEps, _ := episodes.ListByShow(ctx, show.ID)
	var airedID string
	for _, e := range allEps {
		if e.Name == "Aired" {
			airedID = e.ID
		}
	}

	// --- Library entry.
	entry := &model.LibraryEntry{
		UserID:    userID,
		MediaType: model.MediaTypeTV,
		ShowID:    &show.ID,
		CreatedAt: 1,
		UpdatedAt: 1,
	}
	if err := library.Create(ctx, entry); err != nil {
		t.Fatalf("create library: %v", err)
	}

	// --- Before watching anything, progress should be 0/1 (special and
	// future excluded).
	watched, total, err := events.ProgressForShow(ctx, userID, show.ID)
	if err != nil {
		t.Fatalf("progress: %v", err)
	}
	if watched != 0 || total != 1 {
		t.Errorf("initial progress = %d/%d, want 0/1", watched, total)
	}

	// NextUnwatched should return the aired regular episode.
	next, err := events.NextUnwatched(ctx, userID, show.ID)
	if err != nil {
		t.Fatalf("next unwatched: %v", err)
	}
	if next.Name != "Aired" {
		t.Errorf("next = %q, want Aired", next.Name)
	}

	// --- Mark the aired episode watched.
	if err := events.Create(ctx, &model.WatchEvent{
		UserID:    userID,
		EpisodeID: &airedID,
		WatchedAt: 100,
	}); err != nil {
		t.Fatalf("create watch event: %v", err)
	}

	watched, total, _ = events.ProgressForShow(ctx, userID, show.ID)
	if watched != 1 || total != 1 {
		t.Errorf("after watching progress = %d/%d, want 1/1", watched, total)
	}
	if _, err := events.NextUnwatched(ctx, userID, show.ID); err == nil {
		t.Error("expected no more unwatched after watching the only aired regular episode")
	}

	// --- ListContinueWatching should not return this show (all aired
	// regular episodes are done) once we set the manual status.
	watching := model.MediaStatusWatching
	if err := library.SetManualStatus(ctx, entry.ID, &watching); err != nil {
		t.Fatalf("set status: %v", err)
	}
	cw, err := library.ListContinueWatching(ctx, userID, 10)
	if err != nil {
		t.Fatalf("continue watching: %v", err)
	}
	if len(cw) != 0 {
		t.Errorf("expected no continue-watching rows, got %d", len(cw))
	}

	// --- Rate the show.
	if err := ratings.UpsertShow(ctx, userID, show.ID, 9); err != nil {
		t.Fatalf("rate show: %v", err)
	}
	rs, err := ratings.GetShow(ctx, userID, show.ID)
	if err != nil || rs == nil {
		t.Fatalf("get show rating: %v / %v", rs, err)
	}
	if rs.Score != 9 {
		t.Errorf("score = %d, want 9", rs.Score)
	}

	// --- Tag the entry.
	tagID := uuid.NewString()
	_, err = db.ExecContext(ctx,
		`INSERT INTO tag (id, user_id, name, created_at) VALUES (?, ?, 'fantasy', strftime('%s','now'))`,
		tagID, userID)
	if err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	if err := tags.Add(ctx, entry.ID, tagID); err != nil {
		t.Fatalf("add tag: %v", err)
	}
	linked, err := tags.ListByLibrary(ctx, entry.ID)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	if len(linked) != 1 || linked[0].Name != "fantasy" {
		t.Errorf("linked tags = %+v", linked)
	}
}
