package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/google/uuid"
)

// LibraryRepository persists per-user library rows and runs the derived
// status / unwatched-count queries that drive the library grid, dashboard,
// and watched page.
type LibraryRepository struct {
	repo
}

var _ model.LibraryRepository = (*LibraryRepository)(nil)

func NewLibraryRepository(db DBTX) *LibraryRepository {
	return &LibraryRepository{repo{db: db}}
}

func (r *LibraryRepository) Create(ctx context.Context, e *model.LibraryEntry) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	var manual *string
	if e.ManualStatus != nil {
		s := string(*e.ManualStatus)
		manual = &s
	}
	_, err := r.conn(ctx).ExecContext(ctx,
		`INSERT INTO user_library (id, user_id, media_type, show_id, movie_id, manual_status, watched_at, notes, release_notified, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.UserID, string(e.MediaType), e.ShowID, e.MovieID, manual,
		e.WatchedAt, e.Notes, boolToInt(e.ReleaseNotified), e.CreatedAt, e.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.NewAlreadyExists("item already in library")
		}
		return fmt.Errorf("creating library entry: %w", err)
	}
	return nil
}

// Column list used in every LibraryView SELECT. All columns are prefixed
// with their table alias because the joins pull fields from four tables.
const libraryViewCols = `
	l.id, l.user_id, l.media_type, l.show_id, l.movie_id, l.manual_status,
	l.watched_at, l.notes, l.release_notified, l.created_at, l.updated_at,
	s.id, s.tmdb_id, s.title, s.overview, s.poster_path, s.backdrop_path,
	s.first_air_date, s.genres, s.tmdb_status, s.refreshed_at,
	m.id, m.tmdb_id, m.title, m.overview, m.poster_path, m.backdrop_path,
	m.release_date, m.runtime_minutes, m.genres, m.tmdb_status, m.refreshed_at
`

const libraryViewFrom = `
	FROM user_library l
	LEFT JOIN tmdb_show  s ON l.show_id  = s.id
	LEFT JOIN tmdb_movie m ON l.movie_id = m.id
`

func (r *LibraryRepository) GetByID(ctx context.Context, id string) (*model.LibraryView, error) {
	row := r.conn(ctx).QueryRowContext(ctx,
		`SELECT `+libraryViewCols+libraryViewFrom+` WHERE l.id = ?`, id)
	return scanLibraryViewOne(row)
}

func (r *LibraryRepository) GetByShow(ctx context.Context, userID, showID string) (*model.LibraryView, error) {
	row := r.conn(ctx).QueryRowContext(ctx,
		`SELECT `+libraryViewCols+libraryViewFrom+` WHERE l.user_id = ? AND l.show_id = ?`,
		userID, showID)
	return scanLibraryViewOne(row)
}

func (r *LibraryRepository) GetByMovie(ctx context.Context, userID, movieID string) (*model.LibraryView, error) {
	row := r.conn(ctx).QueryRowContext(ctx,
		`SELECT `+libraryViewCols+libraryViewFrom+` WHERE l.user_id = ? AND l.movie_id = ?`,
		userID, movieID)
	return scanLibraryViewOne(row)
}

// GetByTMDBID is a convenience used by the importer — looks up a library
// entry by the catalog TMDB id rather than the internal UUID.
func (r *LibraryRepository) GetByTMDBID(ctx context.Context, userID string, tmdbID int, mediaType model.MediaType) (*model.LibraryView, error) {
	switch mediaType {
	case model.MediaTypeMovie:
		row := r.conn(ctx).QueryRowContext(ctx,
			`SELECT `+libraryViewCols+libraryViewFrom+`
			 WHERE l.user_id = ? AND l.media_type = 'movie' AND m.tmdb_id = ?`,
			userID, tmdbID)
		return scanLibraryViewOne(row)
	case model.MediaTypeTV:
		row := r.conn(ctx).QueryRowContext(ctx,
			`SELECT `+libraryViewCols+libraryViewFrom+`
			 WHERE l.user_id = ? AND l.media_type = 'tv' AND s.tmdb_id = ?`,
			userID, tmdbID)
		return scanLibraryViewOne(row)
	}
	return nil, model.NewInvalidArgument("unknown media type")
}

func (r *LibraryRepository) List(ctx context.Context, userID string, filter model.LibraryFilter, page model.PageRequest) (*model.PageResponse[model.LibraryView], error) {
	page = page.Normalize()
	offset, err := offsetFromToken(page.PageToken)
	if err != nil {
		return nil, err
	}

	where := []string{"l.user_id = ?"}
	args := []any{userID}
	if len(filter.Statuses) > 0 {
		placeholders := make([]string, len(filter.Statuses))
		for i, st := range filter.Statuses {
			placeholders[i] = "?"
			args = append(args, string(st))
		}
		where = append(where, "COALESCE(l.manual_status, 'plan_to_watch') IN ("+strings.Join(placeholders, ",")+")")
	} else if filter.Status != "" {
		where = append(where, "COALESCE(l.manual_status, 'plan_to_watch') = ?")
		args = append(args, string(filter.Status))
	}
	if filter.MediaType != "" {
		where = append(where, "l.media_type = ?")
		args = append(args, string(filter.MediaType))
	}
	if filter.Query != "" {
		where = append(where, "(s.title LIKE ? OR m.title LIKE ?)")
		args = append(args, "%"+filter.Query+"%", "%"+filter.Query+"%")
	}
	if filter.TagID != "" {
		where = append(where, "l.id IN (SELECT library_id FROM library_tag WHERE tag_id = ?)")
		args = append(args, filter.TagID)
	}
	whereClause := strings.Join(where, " AND ")

	var total int
	if err := r.conn(ctx).QueryRowContext(ctx,
		`SELECT COUNT(*) `+libraryViewFrom+` WHERE `+whereClause, args...,
	).Scan(&total); err != nil {
		return nil, fmt.Errorf("counting library: %w", err)
	}

	orderBy, extraSelect, extraArgs := libraryOrderBy(filter, userID)
	queryArgs := append(extraArgs, args...)
	queryArgs = append(queryArgs, page.PageSize, offset)

	query := `SELECT ` + libraryViewCols + extraSelect + libraryViewFrom +
		` WHERE ` + whereClause + ` ORDER BY ` + orderBy + ` LIMIT ? OFFSET ?`

	rows, err := r.conn(ctx).QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("listing library: %w", err)
	}
	defer rows.Close()

	var items []model.LibraryView
	for rows.Next() {
		var v *model.LibraryView
		if filter.SortBy == "unwatched" {
			v, err = scanLibraryViewWithUnwatched(rows)
		} else {
			v, err = scanLibraryView(rows)
		}
		if err != nil {
			return nil, err
		}
		items = append(items, *v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &model.PageResponse[model.LibraryView]{
		Items:         items,
		NextPageToken: nextToken(offset, page.PageSize, total),
		TotalSize:     total,
	}, nil
}

// libraryOrderBy builds the ORDER BY plus any extra SELECT columns the
// chosen sort needs. The unwatched-episodes sort joins a CTE-style LEFT
// JOIN against a per-show unwatched-count subquery.
func libraryOrderBy(filter model.LibraryFilter, userID string) (orderBy, extraSelect string, extraArgs []any) {
	dir := "DESC"
	if strings.EqualFold(filter.SortDir, "asc") {
		dir = "ASC"
	} else if strings.EqualFold(filter.SortDir, "desc") {
		dir = "DESC"
	} else if filter.SortBy == "title" {
		dir = "ASC"
	}

	// Title chooses between show and movie title via COALESCE.
	titleExpr := "COALESCE(s.title, m.title)"
	releaseExpr := "COALESCE(s.first_air_date, m.release_date)"

	switch filter.SortBy {
	case "title":
		return titleExpr + " " + dir + ", l.id", "", nil
	case "release_date":
		return releaseExpr + " " + dir + ", l.id", "", nil
	case "watched_at":
		return "CASE WHEN l.watched_at IS NOT NULL THEN 0 ELSE 1 END, l.watched_at " + dir + ", l.id", "", nil
	case "unwatched":
		// Count specials too per user request — see library_repository
		// comment in Phase C for why this intentionally differs from
		// aired_regular_episodes.
		extraSelect = `, COALESCE(uc.cnt, 0) AS unwatched_count`
		orderBy = "CASE WHEN l.media_type='movie' THEN 1 ELSE 0 END, COALESCE(uc.cnt, 0) " + dir + ", l.id"
		// The LEFT JOIN is injected via a FROM suffix in the List call
		// by way of the extraSelect — but SQLite requires the JOIN to
		// appear in the FROM clause, not SELECT. We emit it as an
		// inline subquery in the SELECT list instead.
		extraSelect = `, (SELECT COUNT(*)
		                   FROM tmdb_episode e
		                   JOIN tmdb_season sn ON e.season_id = sn.id
		                   WHERE sn.show_id = l.show_id
		                     AND e.air_date IS NOT NULL
		                     AND e.air_date <= CAST(strftime('%s','now') AS INTEGER)
		                     AND NOT EXISTS (
		                       SELECT 1 FROM watch_event wp
		                       WHERE wp.episode_id = e.id AND wp.user_id = l.user_id
		                     )
		                  ) AS unwatched_count`
		// uc alias no longer used; rewrite order clause.
		orderBy = "CASE WHEN l.media_type='movie' THEN 1 ELSE 0 END, unwatched_count " + dir + ", l.id"
		return orderBy, extraSelect, nil
	case "", "created_at":
		return "l.created_at " + dir + ", l.id", "", nil
	}
	return "l.created_at " + dir + ", l.id", "", nil
}

// ListContinueWatching returns user_library rows whose derived status is
// "watching" and that still have at least one unwatched aired-regular
// episode (movies in "watching" pass through unconditionally). Single
// query, no per-row recalc, uses the watch_event indexes.
//
// Ordered by the most recent aired episode (or movie release date) DESC
// so the dashboard's "last aired …" badge stays in sync with the
// row order. Doing this in SQL — rather than sorting in Go after the
// LIMIT — guarantees a freshly-aired show isn't pruned by the limit
// before the sort sees it.
func (r *LibraryRepository) ListContinueWatching(ctx context.Context, userID string, limit int) ([]model.LibraryView, error) {
	if limit <= 0 {
		limit = 10
	}
	query := `SELECT ` + libraryViewCols + libraryViewFrom + `
		WHERE l.user_id = ?
		  AND COALESCE(l.manual_status, 'plan_to_watch') = 'watching'
		  AND (
		    l.media_type = 'movie'
		    OR EXISTS (
		      SELECT 1 FROM aired_regular_episodes e
		      JOIN tmdb_season sn ON e.season_id = sn.id
		      WHERE sn.show_id = l.show_id
		        AND NOT EXISTS (
		          SELECT 1 FROM watch_event we
		          WHERE we.episode_id = e.id AND we.user_id = l.user_id
		        )
		    )
		  )
		ORDER BY COALESCE(
		           (SELECT MAX(e.air_date)
		              FROM aired_regular_episodes e
		              JOIN tmdb_season sn ON e.season_id = sn.id
		              WHERE sn.show_id = l.show_id),
		           m.release_date
		         ) IS NULL,
		         COALESCE(
		           (SELECT MAX(e.air_date)
		              FROM aired_regular_episodes e
		              JOIN tmdb_season sn ON e.season_id = sn.id
		              WHERE sn.show_id = l.show_id),
		           m.release_date
		         ) DESC,
		         l.id
		LIMIT ?`

	rows, err := r.conn(ctx).QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing continue watching: %w", err)
	}
	defer rows.Close()

	var out []model.LibraryView
	for rows.Next() {
		v, err := scanLibraryView(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

// ListUnratedWatched returns anything the user has started (has a watch
// event, or a watched_at, or has status='watched') and which does NOT
// have a rating yet. Excludes plan_to_watch and dropped.
func (r *LibraryRepository) ListUnratedWatched(ctx context.Context, userID string, limit int) ([]model.LibraryView, error) {
	if limit <= 0 {
		limit = 10
	}
	query := `SELECT ` + libraryViewCols + libraryViewFrom + `
		WHERE l.user_id = ?
		  AND COALESCE(l.manual_status, 'plan_to_watch') NOT IN ('plan_to_watch', 'dropped')
		  AND (
		    COALESCE(l.manual_status, 'plan_to_watch') = 'watched'
		    OR l.watched_at IS NOT NULL
		    OR EXISTS (
		      SELECT 1 FROM watch_event we
		      WHERE we.user_id = l.user_id
		        AND ((we.episode_id IN (
		               SELECT e.id FROM tmdb_episode e
		               JOIN tmdb_season sn ON e.season_id = sn.id
		               WHERE sn.show_id = l.show_id
		             ))
		             OR we.movie_id = l.movie_id)
		    )
		  )
		  AND NOT EXISTS (
		    SELECT 1 FROM rating_show  rs WHERE rs.user_id = l.user_id AND rs.show_id  = l.show_id
		    UNION ALL
		    SELECT 1 FROM rating_movie rm WHERE rm.user_id = l.user_id AND rm.movie_id = l.movie_id
		  )
		ORDER BY COALESCE(l.watched_at, l.updated_at) DESC
		LIMIT ?`

	rows, err := r.conn(ctx).QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing unrated watched: %w", err)
	}
	defer rows.Close()

	var out []model.LibraryView
	for rows.Next() {
		v, err := scanLibraryView(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

func (r *LibraryRepository) SetManualStatus(ctx context.Context, id string, status *model.MediaStatus) error {
	var val *string
	if status != nil {
		s := string(*status)
		val = &s
	}
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE user_library SET manual_status = ?, updated_at = strftime('%s','now') WHERE id = ?`,
		val, id)
	if err != nil {
		return fmt.Errorf("setting manual status: %w", err)
	}
	return nil
}

func (r *LibraryRepository) UpdateWatchedAt(ctx context.Context, id string, watchedAt *int64) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE user_library SET watched_at = ?, updated_at = strftime('%s','now') WHERE id = ?`,
		watchedAt, id)
	if err != nil {
		return fmt.Errorf("updating watched_at: %w", err)
	}
	return nil
}

func (r *LibraryRepository) UpdateNotes(ctx context.Context, id, notes string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE user_library SET notes = ?, updated_at = strftime('%s','now') WHERE id = ?`,
		notes, id)
	if err != nil {
		return fmt.Errorf("updating notes: %w", err)
	}
	return nil
}

func (r *LibraryRepository) MarkReleaseNotified(ctx context.Context, id string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE user_library SET release_notified = 1, updated_at = strftime('%s','now') WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("marking release notified: %w", err)
	}
	return nil
}

// GetLibraryMap returns a map of "tmdbID:mediaType" → library entry ID
// for the entire user's library. Used by the discovery service to filter
// out recommendations the user has already added.
func (r *LibraryRepository) GetLibraryMap(ctx context.Context, userID string) (map[string]string, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT l.id, l.media_type,
		        COALESCE(s.tmdb_id, 0) AS show_tmdb,
		        COALESCE(m.tmdb_id, 0) AS movie_tmdb
		 FROM user_library l
		 LEFT JOIN tmdb_show  s ON l.show_id  = s.id
		 LEFT JOIN tmdb_movie m ON l.movie_id = m.id
		 WHERE l.user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("getting library map: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, mediaType string
		var showTMDB, movieTMDB int
		if err := rows.Scan(&id, &mediaType, &showTMDB, &movieTMDB); err != nil {
			continue
		}
		tmdbID := showTMDB
		if mediaType == "movie" {
			tmdbID = movieTMDB
		}
		out[fmt.Sprintf("%d:%s", tmdbID, mediaType)] = id
	}
	return out, rows.Err()
}

// ListTopRatedEntries returns the user's highest-rated library entries
// (across both shows and movies), sorted by score descending. Used by
// the discovery service to pick seeds for the recommendation feed.
func (r *LibraryRepository) ListTopRatedEntries(ctx context.Context, userID string, limit int) ([]model.LibraryView, error) {
	if limit <= 0 {
		limit = 10
	}
	query := `SELECT ` + libraryViewCols + libraryViewFrom + `
		LEFT JOIN rating_show  rs ON rs.user_id = l.user_id AND rs.show_id  = l.show_id
		LEFT JOIN rating_movie rm ON rm.user_id = l.user_id AND rm.movie_id = l.movie_id
		WHERE l.user_id = ?
		  AND (rs.score IS NOT NULL OR rm.score IS NOT NULL)
		ORDER BY COALESCE(rs.score, rm.score) DESC, l.updated_at DESC
		LIMIT ?`
	rows, err := r.conn(ctx).QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing top rated: %w", err)
	}
	defer rows.Close()
	var out []model.LibraryView
	for rows.Next() {
		v, err := scanLibraryView(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

// ListPendingReleaseNotifications returns library entries whose catalog
// release_date has passed and that are in a status the user still cares
// about (plan_to_watch / watching). Used by the release notifier job to
// fire "released" webhooks exactly once per entry.
func (r *LibraryRepository) ListPendingReleaseNotifications(ctx context.Context) ([]model.LibraryView, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT `+libraryViewCols+libraryViewFrom+`
		 WHERE l.release_notified = 0
		   AND COALESCE(l.manual_status, 'plan_to_watch') IN ('plan_to_watch','watching')
		   AND (
		     (l.media_type = 'movie' AND m.release_date IS NOT NULL AND m.release_date <= CAST(strftime('%s','now') AS INTEGER))
		     OR
		     (l.media_type = 'tv'    AND s.first_air_date IS NOT NULL AND s.first_air_date <= CAST(strftime('%s','now') AS INTEGER))
		   )`)
	if err != nil {
		return nil, fmt.Errorf("listing pending release notifications: %w", err)
	}
	defer rows.Close()
	var out []model.LibraryView
	for rows.Next() {
		v, err := scanLibraryView(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

func (r *LibraryRepository) Delete(ctx context.Context, id string) error {
	_, err := r.conn(ctx).ExecContext(ctx, `DELETE FROM user_library WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting library entry: %w", err)
	}
	return nil
}

func (r *LibraryRepository) TotalCount(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.conn(ctx).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_library WHERE user_id = ?`, userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("counting library: %w", err)
	}
	return n, nil
}

// scanLibraryViewOne wraps scanLibraryView and translates sql.ErrNoRows
// into the model.NotFound error callers expect.
func scanLibraryViewOne(row scannable) (*model.LibraryView, error) {
	v, err := scanLibraryView(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.NewNotFound("library entry not found")
		}
		return nil, err
	}
	return v, nil
}

// scanLibraryView scans a LibraryView row from the libraryViewCols layout.
// Either the show_id or movie_id side of the LEFT JOIN will be NULL; the
// populated side is materialized into the Show/Movie pointer.
func scanLibraryView(row scannable) (*model.LibraryView, error) {
	var v model.LibraryView
	var e model.LibraryEntry
	var mediaType string
	var manualStatus sql.NullString
	var watchedAt sql.NullInt64
	var showID, movieID sql.NullString
	var releaseNotified int

	var (
		sID, sTitle, sOverview, sPoster, sBackdrop, sGenres, sTStatus sql.NullString
		sTMDBID                                                       sql.NullInt64
		sFirstAir, sRefreshed                                         sql.NullInt64

		mID, mTitle, mOverview, mPoster, mBackdrop, mGenres, mTStatus sql.NullString
		mTMDBID, mRuntime                                             sql.NullInt64
		mRelease, mRefreshed                                          sql.NullInt64
	)

	err := row.Scan(
		&e.ID, &e.UserID, &mediaType, &showID, &movieID, &manualStatus,
		&watchedAt, &e.Notes, &releaseNotified, &e.CreatedAt, &e.UpdatedAt,
		&sID, &sTMDBID, &sTitle, &sOverview, &sPoster, &sBackdrop,
		&sFirstAir, &sGenres, &sTStatus, &sRefreshed,
		&mID, &mTMDBID, &mTitle, &mOverview, &mPoster, &mBackdrop,
		&mRelease, &mRuntime, &mGenres, &mTStatus, &mRefreshed,
	)
	if err != nil {
		return nil, err
	}

	e.MediaType = model.MediaType(mediaType)
	if showID.Valid {
		s := showID.String
		e.ShowID = &s
	}
	if movieID.Valid {
		s := movieID.String
		e.MovieID = &s
	}
	if manualStatus.Valid {
		ms := model.MediaStatus(manualStatus.String)
		e.ManualStatus = &ms
	}
	if watchedAt.Valid {
		w := watchedAt.Int64
		e.WatchedAt = &w
	}
	e.ReleaseNotified = releaseNotified == 1

	v.Entry = e

	if sID.Valid {
		show := model.TMDBShow{
			ID:           sID.String,
			TMDBID:       int(sTMDBID.Int64),
			Title:        sTitle.String,
			Overview:     sOverview.String,
			PosterPath:   sPoster.String,
			BackdropPath: sBackdrop.String,
			Genres:       sGenres.String,
			TMDBStatus:   sTStatus.String,
			RefreshedAt:  sRefreshed.Int64,
		}
		if sFirstAir.Valid {
			f := sFirstAir.Int64
			show.FirstAirDate = &f
		}
		v.Show = &show
	}
	if mID.Valid {
		movie := model.TMDBMovie{
			ID:             mID.String,
			TMDBID:         int(mTMDBID.Int64),
			Title:          mTitle.String,
			Overview:       mOverview.String,
			PosterPath:     mPoster.String,
			BackdropPath:   mBackdrop.String,
			RuntimeMinutes: int(mRuntime.Int64),
			Genres:         mGenres.String,
			TMDBStatus:     mTStatus.String,
			RefreshedAt:    mRefreshed.Int64,
		}
		if mRelease.Valid {
			r := mRelease.Int64
			movie.ReleaseDate = &r
		}
		v.Movie = &movie
	}

	// Effective status: manual override if set, else "plan_to_watch" as
	// a placeholder. The service layer overwrites this with the real
	// derived value when it needs to.
	if e.ManualStatus != nil {
		v.Status = *e.ManualStatus
	} else {
		v.Status = model.MediaStatusPlanToWatch
	}

	return &v, nil
}

// scanLibraryViewWithUnwatched is like scanLibraryView but also pulls the
// trailing unwatched_count column emitted when sorting by unwatched.
func scanLibraryViewWithUnwatched(row scannable) (*model.LibraryView, error) {
	// Reuse the same column ordering by scanning into a combined struct.
	var v model.LibraryView
	var e model.LibraryEntry
	var mediaType string
	var manualStatus sql.NullString
	var watchedAt sql.NullInt64
	var showID, movieID sql.NullString
	var releaseNotified int
	var unwatched int

	var (
		sID, sTitle, sOverview, sPoster, sBackdrop, sGenres, sTStatus sql.NullString
		sTMDBID                                                       sql.NullInt64
		sFirstAir, sRefreshed                                         sql.NullInt64

		mID, mTitle, mOverview, mPoster, mBackdrop, mGenres, mTStatus sql.NullString
		mTMDBID, mRuntime                                             sql.NullInt64
		mRelease, mRefreshed                                          sql.NullInt64
	)

	err := row.Scan(
		&e.ID, &e.UserID, &mediaType, &showID, &movieID, &manualStatus,
		&watchedAt, &e.Notes, &releaseNotified, &e.CreatedAt, &e.UpdatedAt,
		&sID, &sTMDBID, &sTitle, &sOverview, &sPoster, &sBackdrop,
		&sFirstAir, &sGenres, &sTStatus, &sRefreshed,
		&mID, &mTMDBID, &mTitle, &mOverview, &mPoster, &mBackdrop,
		&mRelease, &mRuntime, &mGenres, &mTStatus, &mRefreshed,
		&unwatched,
	)
	if err != nil {
		return nil, err
	}

	e.MediaType = model.MediaType(mediaType)
	if showID.Valid {
		s := showID.String
		e.ShowID = &s
	}
	if movieID.Valid {
		s := movieID.String
		e.MovieID = &s
	}
	if manualStatus.Valid {
		ms := model.MediaStatus(manualStatus.String)
		e.ManualStatus = &ms
	}
	if watchedAt.Valid {
		w := watchedAt.Int64
		e.WatchedAt = &w
	}
	e.ReleaseNotified = releaseNotified == 1
	v.Entry = e
	v.UnwatchedCount = unwatched

	if sID.Valid {
		show := model.TMDBShow{
			ID:           sID.String,
			TMDBID:       int(sTMDBID.Int64),
			Title:        sTitle.String,
			Overview:     sOverview.String,
			PosterPath:   sPoster.String,
			BackdropPath: sBackdrop.String,
			Genres:       sGenres.String,
			TMDBStatus:   sTStatus.String,
			RefreshedAt:  sRefreshed.Int64,
		}
		if sFirstAir.Valid {
			f := sFirstAir.Int64
			show.FirstAirDate = &f
		}
		v.Show = &show
	}
	if mID.Valid {
		movie := model.TMDBMovie{
			ID:             mID.String,
			TMDBID:         int(mTMDBID.Int64),
			Title:          mTitle.String,
			Overview:       mOverview.String,
			PosterPath:     mPoster.String,
			BackdropPath:   mBackdrop.String,
			RuntimeMinutes: int(mRuntime.Int64),
			Genres:         mGenres.String,
			TMDBStatus:     mTStatus.String,
			RefreshedAt:    mRefreshed.Int64,
		}
		if mRelease.Valid {
			r := mRelease.Int64
			movie.ReleaseDate = &r
		}
		v.Movie = &movie
	}
	if e.ManualStatus != nil {
		v.Status = *e.ManualStatus
	} else {
		v.Status = model.MediaStatusPlanToWatch
	}
	return &v, nil
}
