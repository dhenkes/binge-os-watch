package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/dhenkes/binge-os-watch/internal/repository"
	"github.com/google/uuid"
)

// TestLibraryExporter_PaginatesPastMaxPageSize is a regression test for
// the bug where the exporter passed PageSize=100000 to LibraryRepository.List,
// not realising PageRequest.Normalize() silently clamps PageSize to
// MaxPageSize (100). The result was a hard 100-item ceiling on every
// export. The fix paginates through with the actual MaxPageSize and
// follows NextPageToken until exhausted; this test seeds 250 shows so a
// failure to paginate would only return 100.
func TestLibraryExporter_PaginatesPastMaxPageSize(t *testing.T) {
	const seedCount = 250

	// In-memory DB with the new schema applied.
	dsn := fmt.Sprintf("file:export_test_%p?mode=memory&cache=shared", t)
	db, err := repository.NewSQLiteDB(dsn)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()

	// Seed user.
	userID := uuid.NewString()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO users (id, username, password, role, created_at)
		 VALUES (?, ?, ?, 'user', strftime('%s','now'))`,
		userID, "exporter", "hash"); err != nil {
		t.Fatalf("seeding user: %v", err)
	}

	// Seed seedCount catalog shows + matching user_library entries.
	// Direct SQL is faster than going through LibraryService.Add which
	// would hit TMDB.
	for i := 0; i < seedCount; i++ {
		showID := uuid.NewString()
		if _, err := db.ExecContext(ctx,
			`INSERT INTO tmdb_show (id, tmdb_id, title, refreshed_at)
			 VALUES (?, ?, ?, strftime('%s','now'))`,
			showID, 1000+i, fmt.Sprintf("Show %d", i)); err != nil {
			t.Fatalf("seeding show %d: %v", i, err)
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO user_library (id, user_id, media_type, show_id, created_at, updated_at)
			 VALUES (?, ?, 'tv', ?, strftime('%s','now'), strftime('%s','now'))`,
			uuid.NewString(), userID, showID); err != nil {
			t.Fatalf("seeding library entry %d: %v", i, err)
		}
	}

	exporter := NewLibraryExporter(
		repository.NewLibraryRepository(db),
		repository.NewLibraryTagRepository(db),
		repository.NewWatchEventRepository(db),
		repository.NewRatingV2Repository(db),
		repository.NewTMDBSeasonRepository(db),
		repository.NewTMDBEpisodeRepository(db),
		repository.NewTagRepository(db),
	)

	out, err := exporter.Export(ctx, userID)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if got := len(out.Shows); got != seedCount {
		t.Errorf("len(Shows) = %d, want %d (pagination loop is broken — likely silently clamped to MaxPageSize)", got, seedCount)
	}
	// Sanity: assert MaxPageSize hasn't been bumped to >= seedCount in
	// the future, which would mask the regression. If you bump
	// MaxPageSize past 250, raise the seed count too.
	if model.MaxPageSize >= seedCount {
		t.Fatalf("MaxPageSize is %d, which is >= seed count %d — raise the seed count to keep this test meaningful", model.MaxPageSize, seedCount)
	}
}
