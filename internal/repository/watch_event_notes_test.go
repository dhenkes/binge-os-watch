package repository

import (
	"context"
	"testing"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/google/uuid"
)

// TestWatchEventRepository_UpdateLatestNotesForEpisode verifies that
// UpdateLatestNotesForEpisode only touches the most recent watch row for
// the given (user, episode) pair. Earlier rewatches keep whatever notes
// they had at creation time.
func TestWatchEventRepository_UpdateLatestNotesForEpisode(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	userID := uuid.NewString()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO users (id, username, password, role, created_at)
		 VALUES (?, ?, ?, 'user', strftime('%s','now'))`,
		userID, "alice", "hash"); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	shows := NewTMDBShowRepository(db)
	seasons := NewTMDBSeasonRepository(db)
	episodes := NewTMDBEpisodeRepository(db)
	events := NewWatchEventRepository(db)

	show := &model.TMDBShow{TMDBID: 1, Title: "X", RefreshedAt: 1}
	if err := shows.Upsert(ctx, show); err != nil {
		t.Fatalf("show: %v", err)
	}
	if err := seasons.UpsertBatch(ctx, []model.TMDBSeason{
		{ShowID: show.ID, SeasonNumber: 1, Name: "S1", TMDBSeasonID: 10},
	}); err != nil {
		t.Fatalf("season: %v", err)
	}
	all, _ := seasons.ListByShow(ctx, show.ID)
	seasonID := all[0].ID
	air := int64(1)
	if err := episodes.UpsertBatch(ctx, []model.TMDBEpisode{
		{SeasonID: seasonID, TMDBEpisodeID: 1, EpisodeNumber: 1, Name: "E1", AirDate: &air},
	}); err != nil {
		t.Fatalf("ep: %v", err)
	}
	eps, _ := episodes.ListByShow(ctx, show.ID)
	epID := eps[0].ID

	// Two events: an older one and a newer one. Notes should only land
	// on the newer one.
	early := &model.WatchEvent{UserID: userID, EpisodeID: &epID, WatchedAt: 100, Notes: "old"}
	if err := events.Create(ctx, early); err != nil {
		t.Fatalf("early: %v", err)
	}
	late := &model.WatchEvent{UserID: userID, EpisodeID: &epID, WatchedAt: 200, Notes: ""}
	if err := events.Create(ctx, late); err != nil {
		t.Fatalf("late: %v", err)
	}

	ok, err := events.UpdateLatestNotesForEpisode(ctx, userID, epID, "rewatch thoughts")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !ok {
		t.Fatalf("UpdateLatestNotesForEpisode returned ok=false; wanted a row update")
	}

	got, err := events.ListForEpisode(ctx, userID, epID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 events, got %d", len(got))
	}
	// ListForEpisode orders by watched_at DESC, so got[0] is the latest.
	if got[0].Notes != "rewatch thoughts" {
		t.Errorf("latest notes = %q, want %q", got[0].Notes, "rewatch thoughts")
	}
	if got[1].Notes != "old" {
		t.Errorf("old notes got overwritten: %q", got[1].Notes)
	}
}

// TestWatchEventRepository_UpdateLatestNotesForEpisode_None verifies the
// false-return when no event row exists yet.
func TestWatchEventRepository_UpdateLatestNotesForEpisode_None(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	events := NewWatchEventRepository(db)

	ok, err := events.UpdateLatestNotesForEpisode(ctx, "no-user", "no-ep", "x")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ok {
		t.Errorf("UpdateLatestNotesForEpisode on empty table should return ok=false")
	}
}
