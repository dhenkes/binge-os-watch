package repository

import (
	"context"
	"testing"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// seedShowAndSeasons creates a show with two seasons so tests can move
// episodes between them. Returns (showID, seasonAID, seasonBID).
func seedShowAndSeasons(t *testing.T, ctx context.Context, r *TMDBEpisodeRepository) (string, string, string) {
	t.Helper()
	conn := r.conn(ctx)
	const showID = "show-1"
	if _, err := conn.ExecContext(ctx,
		`INSERT INTO tmdb_show (id, tmdb_id, title, refreshed_at) VALUES (?, ?, ?, ?)`,
		showID, 9999, "Test Show", 0,
	); err != nil {
		t.Fatalf("seed show: %v", err)
	}
	const seasonA, seasonB = "season-a", "season-b"
	if _, err := conn.ExecContext(ctx,
		`INSERT INTO tmdb_season (id, show_id, tmdb_season_id, season_number) VALUES (?, ?, ?, ?), (?, ?, ?, ?)`,
		seasonA, showID, 1, 1,
		seasonB, showID, 2, 2,
	); err != nil {
		t.Fatalf("seed seasons: %v", err)
	}
	return showID, seasonA, seasonB
}

// TestUpsertBatch_TMDBIDCollision_AcrossSeasons reproduces the original
// bug: an episode whose tmdb_episode_id is already present (under a
// different season+number, e.g. TMDB renumbered the season) used to fail
// the UNIQUE constraint on tmdb_episode_id because the upsert only
// targeted (season_id, episode_number).
func TestUpsertBatch_TMDBIDCollision_AcrossSeasons(t *testing.T) {
	db := testDB(t)
	r := NewTMDBEpisodeRepository(db)
	ctx := context.Background()

	_, seasonA, seasonB := seedShowAndSeasons(t, ctx, r)

	// First insert: episode 4 in season A.
	if err := r.UpsertBatch(ctx, []model.TMDBEpisode{{
		SeasonID:      seasonA,
		TMDBEpisodeID: 2067,
		EpisodeNumber: 4,
		Name:          "Original",
	}}); err != nil {
		t.Fatalf("initial insert: %v", err)
	}

	// Second insert: same TMDB id, but the episode "moved" to a
	// different season+number. Must reuse the existing row.
	if err := r.UpsertBatch(ctx, []model.TMDBEpisode{{
		SeasonID:      seasonB,
		TMDBEpisodeID: 2067,
		EpisodeNumber: 9,
		Name:          "Renumbered",
	}}); err != nil {
		t.Fatalf("collision upsert: %v", err)
	}

	// Exactly one row, with the new fields, still keyed on the same
	// tmdb_episode_id.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tmdb_episode WHERE tmdb_episode_id = 2067`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("got %d rows for tmdb_episode_id=2067, want 1", count)
	}

	got, err := r.GetByTMDBID(ctx, 2067)
	if err != nil {
		t.Fatalf("GetByTMDBID: %v", err)
	}
	if got.SeasonID != seasonB || got.EpisodeNumber != 9 || got.Name != "Renumbered" {
		t.Errorf("row not updated: season=%q ep=%d name=%q", got.SeasonID, got.EpisodeNumber, got.Name)
	}
}

// TestUpsertBatch_SeasonEpisodeCollision is the other path: a TMDB id
// that doesn't exist yet, but whose (season_id, episode_number) slot is
// already taken by a row carrying a different tmdb_episode_id. The new
// row must take the slot, and the old row must be preserved (its
// watch_event rows still reference it) rather than silently overwritten.
func TestUpsertBatch_SeasonEpisodeCollision(t *testing.T) {
	db := testDB(t)
	r := NewTMDBEpisodeRepository(db)
	ctx := context.Background()

	_, seasonA, _ := seedShowAndSeasons(t, ctx, r)

	if err := r.UpsertBatch(ctx, []model.TMDBEpisode{{
		SeasonID:      seasonA,
		TMDBEpisodeID: 100,
		EpisodeNumber: 1,
		Name:          "Old",
	}}); err != nil {
		t.Fatalf("initial: %v", err)
	}

	// Different TMDB id, same slot — TMDB swapped out the canonical
	// episode for that season+number.
	if err := r.UpsertBatch(ctx, []model.TMDBEpisode{{
		SeasonID:      seasonA,
		TMDBEpisodeID: 200,
		EpisodeNumber: 1,
		Name:          "New",
	}}); err != nil {
		t.Fatalf("collision: %v", err)
	}

	// New row owns the slot with the correct name.
	got, err := r.GetByTMDBID(ctx, 200)
	if err != nil {
		t.Fatalf("GetByTMDBID(200): %v", err)
	}
	if got.Name != "New" || got.EpisodeNumber != 1 {
		t.Errorf("got name=%q ep=%d, want name=New ep=1", got.Name, got.EpisodeNumber)
	}

	// Old row is preserved (would still resolve any watch_event FK) but
	// parked out of the way at a negative episode_number.
	old, err := r.GetByTMDBID(ctx, 100)
	if err != nil {
		t.Fatalf("GetByTMDBID(100): %v", err)
	}
	if old.EpisodeNumber >= 0 {
		t.Errorf("old ep number = %d, want negative", old.EpisodeNumber)
	}
}

// TestUpsertBatch_SwapEpisodeNumbers is the original log error: two
// existing episodes in the same season swap their episode_number in a
// single batch. Without the park pre-pass, updating the first row trips
// the (season_id, episode_number) UNIQUE constraint because the second
// row is still sitting in the slot the first one is moving into.
func TestUpsertBatch_SwapEpisodeNumbers(t *testing.T) {
	db := testDB(t)
	r := NewTMDBEpisodeRepository(db)
	ctx := context.Background()

	_, seasonA, _ := seedShowAndSeasons(t, ctx, r)

	// Seed two episodes occupying slots 4 and 26.
	if err := r.UpsertBatch(ctx, []model.TMDBEpisode{
		{SeasonID: seasonA, TMDBEpisodeID: 2067, EpisodeNumber: 4, Name: "Was4"},
		{SeasonID: seasonA, TMDBEpisodeID: 3000, EpisodeNumber: 26, Name: "Was26"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// TMDB now reports the same two episodes with their numbers swapped.
	if err := r.UpsertBatch(ctx, []model.TMDBEpisode{
		{SeasonID: seasonA, TMDBEpisodeID: 2067, EpisodeNumber: 26, Name: "Was4"},
		{SeasonID: seasonA, TMDBEpisodeID: 3000, EpisodeNumber: 4, Name: "Was26"},
	}); err != nil {
		t.Fatalf("swap upsert: %v", err)
	}

	a, err := r.GetByTMDBID(ctx, 2067)
	if err != nil {
		t.Fatalf("get 2067: %v", err)
	}
	if a.EpisodeNumber != 26 {
		t.Errorf("ep 2067 number = %d, want 26", a.EpisodeNumber)
	}
	b, err := r.GetByTMDBID(ctx, 3000)
	if err != nil {
		t.Fatalf("get 3000: %v", err)
	}
	if b.EpisodeNumber != 4 {
		t.Errorf("ep 3000 number = %d, want 4", b.EpisodeNumber)
	}

	// Sanity: total row count is still 2 — no parked-and-forgotten rows.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tmdb_episode WHERE season_id = ?`, seasonA).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Errorf("row count = %d, want 2", count)
	}
}

// TestUpsertBatch_StaleRowInTargetSlot reproduces the second failure
// mode: a row TMDB no longer reports (so it isn't in the batch) is
// sitting in the (season_id, episode_number) slot the batch wants for a
// different episode. We can't delete the stale row because watch_event
// rows reference it, but it must be moved out of the slot.
func TestUpsertBatch_StaleRowInTargetSlot(t *testing.T) {
	db := testDB(t)
	r := NewTMDBEpisodeRepository(db)
	ctx := context.Background()

	_, seasonA, _ := seedShowAndSeasons(t, ctx, r)

	// Seed a stale row at (S, 26) with a tmdb id that won't appear in
	// the upcoming batch.
	if err := r.UpsertBatch(ctx, []model.TMDBEpisode{
		{SeasonID: seasonA, TMDBEpisodeID: 9999, EpisodeNumber: 26, Name: "Stale"},
	}); err != nil {
		t.Fatalf("seed stale: %v", err)
	}

	// Refresh batch wants ep 2067 at (S, 26). The stale 9999 row blocks
	// the slot.
	if err := r.UpsertBatch(ctx, []model.TMDBEpisode{
		{SeasonID: seasonA, TMDBEpisodeID: 2067, EpisodeNumber: 26, Name: "Fresh"},
	}); err != nil {
		t.Fatalf("upsert with stale blocker: %v", err)
	}

	// 2067 occupies the slot now.
	got, err := r.GetByTMDBID(ctx, 2067)
	if err != nil {
		t.Fatalf("get 2067: %v", err)
	}
	if got.EpisodeNumber != 26 {
		t.Errorf("ep 2067 number = %d, want 26", got.EpisodeNumber)
	}
	// The stale row is preserved (watch_event references would still
	// resolve) but parked at a negative episode_number so it doesn't
	// occupy the slot.
	stale, err := r.GetByTMDBID(ctx, 9999)
	if err != nil {
		t.Fatalf("get 9999: %v", err)
	}
	if stale.EpisodeNumber >= 0 {
		t.Errorf("stale ep number = %d, want negative", stale.EpisodeNumber)
	}
}

// TestUpsertBatch_FreshInsert is the boring happy path — make sure the
// new collision-handling code didn't regress the simplest case.
func TestUpsertBatch_FreshInsert(t *testing.T) {
	db := testDB(t)
	r := NewTMDBEpisodeRepository(db)
	ctx := context.Background()

	_, seasonA, _ := seedShowAndSeasons(t, ctx, r)

	eps := []model.TMDBEpisode{
		{SeasonID: seasonA, TMDBEpisodeID: 1, EpisodeNumber: 1, Name: "Pilot"},
		{SeasonID: seasonA, TMDBEpisodeID: 2, EpisodeNumber: 2, Name: "Second"},
	}
	if err := r.UpsertBatch(ctx, eps); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	for i := range eps {
		if eps[i].ID == "" {
			t.Errorf("episode %d ID not populated", i)
		}
	}
	got, err := r.ListBySeason(ctx, seasonA)
	if err != nil {
		t.Fatalf("ListBySeason: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d rows, want 2", len(got))
	}
}
