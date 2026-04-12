package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/dhenkes/binge-os-watch/internal/repository"
)

// TestWatchService_WatchUpToEpisodeWithDate covers the three date modes
// the handler wires: "today" (all events stamped now), "release" (each
// event stamped with its own air_date), and "custom" (all events stamped
// with a caller-picked unix timestamp).
func TestWatchService_WatchUpToEpisodeWithDate(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		custom  int64
		now     int64
		expectA int64 // expected watched_at for the first aired episode
		expectB int64 // expected watched_at for the target episode
	}{
		{name: "today", mode: "today", now: 5000, expectA: 5000, expectB: 5000},
		{name: "release", mode: "release", now: 5000, expectA: 1000, expectB: 2000},
		{name: "custom", mode: "custom", custom: 7777, now: 5000, expectA: 7777, expectB: 7777},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, epAID, epBID := seedWatchServiceTest(t, tc.now)
			if err := svc.WatchUpToEpisodeWithDate(
				context.Background(), "alice", epBID, tc.mode, tc.custom,
			); err != nil {
				t.Fatalf("WatchUpToEpisodeWithDate: %v", err)
			}

			gotA := latestWatchedAt(t, svc, "alice", epAID)
			if gotA != tc.expectA {
				t.Errorf("episode A watched_at = %d, want %d", gotA, tc.expectA)
			}
			gotB := latestWatchedAt(t, svc, "alice", epBID)
			if gotB != tc.expectB {
				t.Errorf("episode B watched_at = %d, want %d", gotB, tc.expectB)
			}
		})
	}
}

// TestWatchService_WatchUpToEpisodeWithDate_SkipsExisting asserts that
// episodes the user already watched don't get a second event appended by
// the bulk mark (i.e. we don't silently create rewatches).
func TestWatchService_WatchUpToEpisodeWithDate_SkipsExisting(t *testing.T) {
	svc, events, epAID, epBID := seedWatchServiceTest(t, 5000)
	ctx := context.Background()

	// Pre-mark A watched.
	if err := events.Create(ctx, &model.WatchEvent{
		UserID: "alice", EpisodeID: &epAID, WatchedAt: 42,
	}); err != nil {
		t.Fatalf("seed A watched: %v", err)
	}
	if err := svc.WatchUpToEpisodeWithDate(ctx, "alice", epBID, "today", 0); err != nil {
		t.Fatalf("WatchUpToEpisodeWithDate: %v", err)
	}

	// A should still have exactly one event, with watched_at=42.
	all, _ := events.ListForEpisode(ctx, "alice", epAID)
	if len(all) != 1 {
		t.Errorf("episode A events = %d, want 1 (no rewatch)", len(all))
	}
	if len(all) > 0 && all[0].WatchedAt != 42 {
		t.Errorf("episode A watched_at = %d, want 42", all[0].WatchedAt)
	}
}

// seedWatchServiceTest builds a WatchServiceImpl backed by real repos on
// an in-memory DB with one user, one show, one regular season, and two
// aired episodes: A (air=1000) and B (air=2000, the target).
func seedWatchServiceTest(t *testing.T, now int64) (
	svc *WatchServiceImpl,
	events *repository.WatchEventRepository,
	epAID, epBID string,
) {
	t.Helper()
	dsn := fmt.Sprintf("file:watch_svc_%p?mode=memory&cache=shared", t)
	db, err := repository.NewSQLiteDB(dsn)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()

	if _, err := db.ExecContext(ctx,
		`INSERT INTO users (id, username, password, role, created_at)
		 VALUES ('alice', 'alice', 'hash', 'user', strftime('%s','now'))`,
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	shows := repository.NewTMDBShowRepository(db)
	seasons := repository.NewTMDBSeasonRepository(db)
	episodes := repository.NewTMDBEpisodeRepository(db)
	library := repository.NewLibraryRepository(db)
	events = repository.NewWatchEventRepository(db)

	show := &model.TMDBShow{TMDBID: 1, Title: "Show", RefreshedAt: 1}
	if err := shows.Upsert(ctx, show); err != nil {
		t.Fatalf("show: %v", err)
	}
	if err := seasons.UpsertBatch(ctx, []model.TMDBSeason{
		{ShowID: show.ID, SeasonNumber: 1, Name: "S1", TMDBSeasonID: 10},
	}); err != nil {
		t.Fatalf("season: %v", err)
	}
	seasonList, _ := seasons.ListByShow(ctx, show.ID)
	seasonID := seasonList[0].ID

	airA := int64(1000)
	airB := int64(2000)
	if err := episodes.UpsertBatch(ctx, []model.TMDBEpisode{
		{SeasonID: seasonID, TMDBEpisodeID: 1, EpisodeNumber: 1, Name: "A", AirDate: &airA},
		{SeasonID: seasonID, TMDBEpisodeID: 2, EpisodeNumber: 2, Name: "B", AirDate: &airB},
	}); err != nil {
		t.Fatalf("episodes: %v", err)
	}
	all, _ := episodes.ListByShow(ctx, show.ID)
	for _, e := range all {
		switch e.Name {
		case "A":
			epAID = e.ID
		case "B":
			epBID = e.ID
		}
	}

	entry := &model.LibraryEntry{
		UserID: "alice", MediaType: model.MediaTypeTV, ShowID: &show.ID,
	}
	if err := library.Create(ctx, entry); err != nil {
		t.Fatalf("library entry: %v", err)
	}

	svc = NewWatchService(events, library, seasons, episodes)
	svc.now = func() int64 { return now }
	return svc, events, epAID, epBID
}

func latestWatchedAt(t *testing.T, svc *WatchServiceImpl, userID, epID string) int64 {
	t.Helper()
	ev, err := svc.events.LatestForEpisode(context.Background(), userID, epID)
	if err != nil {
		t.Fatalf("LatestForEpisode(%s): %v", epID, err)
	}
	return ev.WatchedAt
}
