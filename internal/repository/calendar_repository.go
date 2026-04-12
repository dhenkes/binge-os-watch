package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// CalendarRepository handles calendar-specific queries against the
// Option B schema. release/air dates live as INTEGER unix-seconds in the
// catalog tables; we format them back to YYYY-MM-DD strings for the
// model.CalendarEntry contract used by the templates.
type CalendarRepository struct {
	repo
}

func NewCalendarRepository(db DBTX) *CalendarRepository {
	return &CalendarRepository{repo{db: db}}
}

// Upcoming returns future releases sorted ascending by release/air date.
func (r *CalendarRepository) Upcoming(ctx context.Context, userID string, filter model.CalendarFilter) ([]model.CalendarEntry, error) {
	now := time.Now().UTC()
	from := startOfDay(now).Unix()
	to := r.dateCutoffUnix(filter.Range, now)

	var entries []model.CalendarEntry

	if filter.MediaType == "" || filter.MediaType == model.MediaTypeMovie {
		movies, err := r.queryMovies(ctx, userID, filter, from, to, "tm.release_date ASC")
		if err != nil {
			return nil, err
		}
		entries = append(entries, movies...)
	}

	if filter.MediaType == "" || filter.MediaType == model.MediaTypeTV {
		episodes, err := r.queryEpisodes(ctx, userID, filter, from, to, "te.air_date ASC")
		if err != nil {
			return nil, err
		}
		entries = append(entries, episodes...)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ReleaseDate < entries[j].ReleaseDate
	})

	return entries, nil
}

// RecentlyReleased returns items aired in the configured range, unwatched first.
func (r *CalendarRepository) RecentlyReleased(ctx context.Context, userID string, filter model.CalendarFilter) ([]model.CalendarEntry, error) {
	now := time.Now().UTC()
	to := startOfDay(now).Unix()
	from := r.recentCutoffUnix(filter.Range, now)

	var entries []model.CalendarEntry

	if filter.MediaType == "" || filter.MediaType == model.MediaTypeMovie {
		movies, err := r.queryMovies(ctx, userID, filter, from, to, "tm.release_date DESC")
		if err != nil {
			return nil, err
		}
		for _, m := range movies {
			if !m.Watched {
				entries = append(entries, m)
			}
		}
	}

	if filter.MediaType == "" || filter.MediaType == model.MediaTypeTV {
		episodes, err := r.queryUnwatchedEpisodes(ctx, userID, filter, from, to)
		if err != nil {
			return nil, err
		}
		entries = append(entries, episodes...)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ReleaseDate > entries[j].ReleaseDate
	})

	return entries, nil
}

// queryMovies pulls library movies whose tmdb_movie.release_date falls in
// [fromUnix, toUnix].
func (r *CalendarRepository) queryMovies(ctx context.Context, userID string, filter model.CalendarFilter,
	fromUnix, toUnix int64, orderBy string) ([]model.CalendarEntry, error) {

	where := []string{
		"ul.user_id = ?",
		"ul.media_type = 'movie'",
		"tm.release_date IS NOT NULL",
		"tm.release_date >= ?",
		"tm.release_date <= ?",
	}
	args := []any{userID, fromUnix, toUnix}

	if filter.Status != "" {
		where = append(where, "COALESCE(ul.manual_status,'plan_to_watch') = ?")
		args = append(args, string(filter.Status))
	}

	query := fmt.Sprintf(`
		SELECT ul.id, tm.title, ul.media_type, tm.poster_path,
		       strftime('%%Y-%%m-%%d', tm.release_date, 'unixepoch'),
		       COALESCE(ul.manual_status, '')
		FROM user_library ul
		JOIN tmdb_movie tm ON tm.id = ul.movie_id
		WHERE %s
		ORDER BY %s`, strings.Join(where, " AND "), orderBy)

	rows, err := r.conn(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying calendar movies: %w", err)
	}
	defer rows.Close()

	var entries []model.CalendarEntry
	for rows.Next() {
		var e model.CalendarEntry
		var mt, status string
		if err := rows.Scan(&e.MediaID, &e.MediaTitle, &mt, &e.PosterPath, &e.ReleaseDate, &status); err != nil {
			return nil, fmt.Errorf("scanning calendar movie: %w", err)
		}
		e.MediaType = model.MediaType(mt)
		e.Watched = status == string(model.MediaStatusWatched)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// queryEpisodes pulls TV episodes for shows in the user's library that
// air in [fromUnix, toUnix].
func (r *CalendarRepository) queryEpisodes(ctx context.Context, userID string, filter model.CalendarFilter,
	fromUnix, toUnix int64, orderBy string) ([]model.CalendarEntry, error) {

	where := []string{
		"ul.user_id = ?",
		"ul.media_type = 'tv'",
		"te.air_date IS NOT NULL",
		"te.air_date >= ?",
		"te.air_date <= ?",
		"ts.season_number > 0",
	}
	args := []any{userID, fromUnix, toUnix}

	if filter.Status != "" {
		where = append(where, "COALESCE(ul.manual_status,'plan_to_watch') = ?")
		args = append(args, string(filter.Status))
	}

	query := fmt.Sprintf(`
		SELECT ul.id, tsh.title, ul.media_type, tsh.poster_path,
		       strftime('%%Y-%%m-%%d', te.air_date, 'unixepoch'),
		       'S' || printf('%%02d', ts.season_number) || 'E' || printf('%%02d', te.episode_number) || ' - ' || te.name,
		       te.id
		FROM user_library ul
		JOIN tmdb_show tsh   ON tsh.id = ul.show_id
		JOIN tmdb_season ts  ON ts.show_id = tsh.id
		JOIN tmdb_episode te ON te.season_id = ts.id
		WHERE %s
		ORDER BY %s`, strings.Join(where, " AND "), orderBy)

	rows, err := r.conn(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying calendar episodes: %w", err)
	}
	defer rows.Close()

	var entries []model.CalendarEntry
	for rows.Next() {
		var e model.CalendarEntry
		var mt string
		if err := rows.Scan(&e.MediaID, &e.MediaTitle, &mt, &e.PosterPath, &e.ReleaseDate, &e.EpisodeInfo, &e.EpisodeID); err != nil {
			return nil, fmt.Errorf("scanning calendar episode: %w", err)
		}
		e.MediaType = model.MediaType(mt)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// queryUnwatchedEpisodes is queryEpisodes with a NOT EXISTS filter that
// drops episodes the user has already logged a watch_event for.
func (r *CalendarRepository) queryUnwatchedEpisodes(ctx context.Context, userID string, filter model.CalendarFilter,
	fromUnix, toUnix int64) ([]model.CalendarEntry, error) {

	where := []string{
		"ul.user_id = ?",
		"ul.media_type = 'tv'",
		"te.air_date IS NOT NULL",
		"te.air_date >= ?",
		"te.air_date < ?",
		"ts.season_number > 0",
		"NOT EXISTS (SELECT 1 FROM watch_event we WHERE we.episode_id = te.id AND we.user_id = ?)",
	}
	args := []any{userID, fromUnix, toUnix, userID}

	if filter.Status != "" {
		where = append(where, "COALESCE(ul.manual_status,'plan_to_watch') = ?")
		args = append(args, string(filter.Status))
	}

	query := fmt.Sprintf(`
		SELECT ul.id, tsh.title, ul.media_type, tsh.poster_path,
		       strftime('%%Y-%%m-%%d', te.air_date, 'unixepoch'),
		       'S' || printf('%%02d', ts.season_number) || 'E' || printf('%%02d', te.episode_number) || ' - ' || te.name,
		       te.id
		FROM user_library ul
		JOIN tmdb_show tsh   ON tsh.id = ul.show_id
		JOIN tmdb_season ts  ON ts.show_id = tsh.id
		JOIN tmdb_episode te ON te.season_id = ts.id
		WHERE %s
		ORDER BY te.air_date DESC`, strings.Join(where, " AND "))

	rows, err := r.conn(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying unwatched episodes: %w", err)
	}
	defer rows.Close()

	var entries []model.CalendarEntry
	for rows.Next() {
		var e model.CalendarEntry
		var mt string
		if err := rows.Scan(&e.MediaID, &e.MediaTitle, &mt, &e.PosterPath, &e.ReleaseDate, &e.EpisodeInfo, &e.EpisodeID); err != nil {
			return nil, fmt.Errorf("scanning unwatched episode: %w", err)
		}
		e.MediaType = model.MediaType(mt)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *CalendarRepository) dateCutoffUnix(rangeStr string, now time.Time) int64 {
	switch rangeStr {
	case "7d":
		return startOfDay(now.AddDate(0, 0, 8)).Unix() - 1
	case "30d":
		return startOfDay(now.AddDate(0, 0, 31)).Unix() - 1
	case "90d":
		return startOfDay(now.AddDate(0, 0, 91)).Unix() - 1
	default:
		return 1<<62 - 1
	}
}

func (r *CalendarRepository) recentCutoffUnix(rangeStr string, now time.Time) int64 {
	switch rangeStr {
	case "7d":
		return startOfDay(now.AddDate(0, 0, -7)).Unix()
	case "30d":
		return startOfDay(now.AddDate(0, 0, -30)).Unix()
	case "90d":
		return startOfDay(now.AddDate(0, 0, -90)).Unix()
	default:
		return 0
	}
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
