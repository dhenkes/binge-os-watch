package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/dhenkes/binge-os-watch/internal/i18n"
	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/dhenkes/binge-os-watch/internal/service"
	"github.com/dhenkes/binge-os-watch/internal/tmdb"
	"github.com/dhenkes/binge-os-watch/internal/transport/http/middleware"
)

// PageHandler serves server-rendered HTML pages against the Option B schema.
type PageHandler struct {
	pages      map[string]*template.Template
	users      model.UserService
	sessionMgr model.SessionManager

	// Library + catalog
	library       model.LibraryService
	libraryRepo   model.LibraryRepository
	watch         model.WatchService
	events        model.WatchEventRepository
	shows         model.TMDBShowRepository
	movies        model.TMDBMovieRepository
	seasons       model.TMDBSeasonRepository
	episodes      model.TMDBEpisodeRepository
	ratings       model.RatingServiceV2
	ratingRepo    model.RatingRepositoryV2
	tags          model.TagService
	tagRepo       model.TagRepository
	libraryTag    model.LibraryTagRepository
	calendar      model.CalendarService
	discovery     model.DiscoveryService
	keywords      model.KeywordWatchService
	stats         model.StatsService
	dismissedRepo model.DismissedRecommendationRepository
	importRunner  *service.LibraryImportRunner
	tmdbJobRepo   model.TMDBJobRepository
	tmdbJobRunner *service.TMDBJobRunner
	webhookRepo   model.WebhookRepository
	webhookSvc    model.WebhookService
	icsSvc        model.ICSService
	traktSvc      model.TraktService
	tmdbClient    *tmdb.Client

	disableRegistration bool
	baseURL             string
	inlineCSS           template.CSS
	defaultRegion       string

	// In-memory cache for fully-resolved preview page data. A preview
	// visit for a 26-season show fires 28+ TMDB calls; the cache turns
	// every repeat within the TTL into a map lookup.
	previews *previewCache
}

// SetTMDBJobHandles wires the tmdb_job admin surface after construction
// (the runner has a cycle with LibraryService, so it's set via a setter
// just like SetTMDBJobRunner on the service side).
func (h *PageHandler) SetTMDBJobHandles(repo model.TMDBJobRepository, runner *service.TMDBJobRunner) {
	h.tmdbJobRepo = repo
	h.tmdbJobRunner = runner
}

// NewPageHandler constructs a PageHandler wired to the Option B services.
func NewPageHandler(
	tmplFS fs.FS,
	staticFS fs.FS,
	users model.UserService,
	sessionMgr model.SessionManager,
	library model.LibraryService,
	libraryRepo model.LibraryRepository,
	watch model.WatchService,
	events model.WatchEventRepository,
	shows model.TMDBShowRepository,
	movies model.TMDBMovieRepository,
	seasons model.TMDBSeasonRepository,
	episodes model.TMDBEpisodeRepository,
	ratings model.RatingServiceV2,
	ratingRepo model.RatingRepositoryV2,
	tags model.TagService,
	tagRepo model.TagRepository,
	libraryTag model.LibraryTagRepository,
	calendar model.CalendarService,
	discovery model.DiscoveryService,
	keywords model.KeywordWatchService,
	stats model.StatsService,
	dismissedRepo model.DismissedRecommendationRepository,
	importRunner *service.LibraryImportRunner,
	webhookRepo model.WebhookRepository,
	webhookSvc model.WebhookService,
	icsSvc model.ICSService,
	traktSvc model.TraktService,
	tmdbClient *tmdb.Client,
	disableRegistration bool,
	baseURL string,
	defaultRegion string,
) *PageHandler {
	cssBytes, _ := fs.ReadFile(staticFS, "style.css")
	css := template.CSS(cssBytes)

	funcMap := template.FuncMap{
		"statusBadge": func(s model.MediaStatus) string {
			switch s {
			case model.MediaStatusWatching:
				return "badge-watching"
			case model.MediaStatusPlanToWatch:
				return "badge-plan"
			case model.MediaStatusWatched:
				return "badge-watched"
			case model.MediaStatusOnHold:
				return "badge-on-hold"
			case model.MediaStatusDropped:
				return "badge-dropped"
			default:
				return ""
			}
		},
		// year accepts either a *int64 unix-seconds (new schema) or a
		// legacy "YYYY-MM-DD" string. Returns "" for unknown.
		"year": func(v any) string {
			switch t := v.(type) {
			case *int64:
				if t == nil || *t == 0 {
					return ""
				}
				return time.Unix(*t, 0).UTC().Format("2006")
			case int64:
				if t == 0 {
					return ""
				}
				return time.Unix(t, 0).UTC().Format("2006")
			case string:
				if len(t) >= 4 {
					return t[:4]
				}
			}
			return ""
		},
		"imgURL": func(size, path string) string {
			if path == "" {
				return ""
			}
			return fmt.Sprintf("/api/v1/images/%s%s", size, path)
		},
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i + 1
			}
			return s
		},
		"trunc": func(n int, s string) string {
			runes := []rune(s)
			if len(runes) <= n {
				return s
			}
			return string(runes[:n]) + "…"
		},
		"add":  func(a, b int) int { return a + b },
		"list": func(args ...any) []any { return args },
		"pct": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a * 100 / b
		},
		"derefInt": func(p *int) int {
			if p == nil {
				return 0
			}
			return *p
		},
		"formatDuration": func(minutes int) string {
			if minutes <= 0 {
				return "0m"
			}
			d := minutes / (60 * 24)
			h := (minutes % (60 * 24)) / 60
			m := minutes % 60
			if d > 0 {
				return fmt.Sprintf("%dd %dh", d, h)
			}
			if h > 0 {
				return fmt.Sprintf("%dh %dm", h, m)
			}
			return fmt.Sprintf("%dm", m)
		},
		"fmtRating": func(f float64) string {
			return fmt.Sprintf("%.1f", f)
		},
		"statusLabel": func(s model.MediaStatus) string {
			return "media." + string(s)
		},
		"fmtDateInput": func(v any) string {
			switch t := v.(type) {
			case *int64:
				if t == nil || *t == 0 {
					return ""
				}
				return time.Unix(*t, 0).UTC().Format("2006-01-02")
			case int64:
				if t == 0 {
					return ""
				}
				return time.Unix(t, 0).UTC().Format("2006-01-02")
			}
			return ""
		},
		"fmtDate": func(v any) string {
			switch t := v.(type) {
			case *int64:
				if t == nil || *t == 0 {
					return ""
				}
				return time.Unix(*t, 0).UTC().Format("Jan 2, 2006")
			case int64:
				if t == 0 {
					return ""
				}
				return time.Unix(t, 0).UTC().Format("Jan 2, 2006")
			case time.Time:
				if t.IsZero() {
					return ""
				}
				return t.Format("Jan 2, 2006")
			case *time.Time:
				if t == nil || t.IsZero() {
					return ""
				}
				return t.Format("Jan 2, 2006")
			}
			return ""
		},
	}

	parse := func(name string) *template.Template {
		return template.Must(template.New("").Funcs(funcMap).ParseFS(
			tmplFS, "templates/layout.html", "templates/"+name+".html",
		))
	}

	pages := map[string]*template.Template{
		"login":          parse("login"),
		"logout":         parse("logout"),
		"library":        parse("library"),
		"menu":           parse("menu"),
		"media_detail":   parse("media_detail"),
		"search":         parse("search"),
		"settings":       parse("settings"),
		"calendar":       parse("calendar"),
		"discover":       parse("discover"),
		"suggestions":    parse("suggestions"),
		"keywords":       parse("keywords"),
		"admin":          parse("admin"),
		"tags":           parse("tags"),
		"episode":        parse("episode"),
		"watched":        parse("watched"),
		"preview":        parse("preview"),
		"webhooks":       parse("webhooks"),
		"webhook_detail": parse("webhook_detail"),
		"webhook_edit":   parse("webhook_edit"),
	}

	return &PageHandler{
		pages:               pages,
		users:               users,
		sessionMgr:          sessionMgr,
		library:             library,
		libraryRepo:         libraryRepo,
		watch:               watch,
		events:              events,
		shows:               shows,
		movies:              movies,
		seasons:             seasons,
		episodes:            episodes,
		ratings:             ratings,
		ratingRepo:          ratingRepo,
		tags:                tags,
		tagRepo:             tagRepo,
		libraryTag:          libraryTag,
		calendar:            calendar,
		discovery:           discovery,
		keywords:            keywords,
		stats:               stats,
		dismissedRepo:       dismissedRepo,
		importRunner:        importRunner,
		webhookRepo:         webhookRepo,
		webhookSvc:          webhookSvc,
		icsSvc:              icsSvc,
		traktSvc:            traktSvc,
		tmdbClient:          tmdbClient,
		disableRegistration: disableRegistration,
		baseURL:             baseURL,
		inlineCSS:           css,
		defaultRegion:       defaultRegion,
		previews:            newPreviewCache(30 * time.Minute),
	}
}

// LibraryCard is a flattened shape for templates that render a library card.
// Keeps the template markup simple — no need to branch on show vs movie.
//
// Refreshing is true while the background TMDB catalog job is still in
// flight for this row (catalog row has RefreshedAt=0, the placeholder
// marker set by LibraryService.AddWithStub). Templates show a small
// "refreshing" badge instead of the normal status chip.
type LibraryCard struct {
	ID             string
	Title          string
	Overview       string
	PosterPath     string
	BackdropPath   string
	MediaType      model.MediaType
	Status         model.MediaStatus
	ReleaseDate    *int64
	RuntimeMinutes int
	Genres         string
	WatchedAt      *int64
	Notes          string
	UnwatchedCount int
	Refreshing     bool
}

func toCard(v model.LibraryView) LibraryCard {
	c := LibraryCard{
		ID:             v.Entry.ID,
		MediaType:      v.Entry.MediaType,
		Status:         viewStatus(v),
		WatchedAt:      v.Entry.WatchedAt,
		Notes:          v.Entry.Notes,
		UnwatchedCount: v.UnwatchedCount,
	}
	if v.Show != nil {
		c.Title = v.Show.Title
		c.Overview = v.Show.Overview
		c.PosterPath = v.Show.PosterPath
		c.BackdropPath = v.Show.BackdropPath
		c.ReleaseDate = v.Show.FirstAirDate
		c.Genres = v.Show.Genres
		c.Refreshing = v.Show.RefreshedAt == 0
	}
	if v.Movie != nil {
		c.Title = v.Movie.Title
		c.Overview = v.Movie.Overview
		c.PosterPath = v.Movie.PosterPath
		c.BackdropPath = v.Movie.BackdropPath
		c.ReleaseDate = v.Movie.ReleaseDate
		c.RuntimeMinutes = v.Movie.RuntimeMinutes
		c.Genres = v.Movie.Genres
		c.Refreshing = v.Movie.RefreshedAt == 0
	}
	return c
}

func toCards(views []model.LibraryView) []LibraryCard {
	out := make([]LibraryCard, len(views))
	for i, v := range views {
		out[i] = toCard(v)
	}
	return out
}

// anyRefreshing reports whether at least one card is still waiting on
// the background TMDB catalog fetch. Used to turn on the layout's
// meta-refresh so placeholder rows heal without a manual F5.
func anyRefreshing(cards []LibraryCard) bool {
	for _, c := range cards {
		if c.Refreshing {
			return true
		}
	}
	return false
}

// viewStatus returns the effective status of a library entry — the manual
// override if the user has set one, otherwise "plan_to_watch" as a safe
// default. The watch service keeps manual_status in sync with derived
// status so list queries can rely on it alone.
func viewStatus(v model.LibraryView) model.MediaStatus {
	if v.Entry.ManualStatus != nil {
		return *v.Entry.ManualStatus
	}
	return model.MediaStatusPlanToWatch
}

// requireAuth validates the session and returns userID, user, settings.
// If unauthenticated, it redirects to /login and returns ok=false.
func (h *PageHandler) requireAuth(w http.ResponseWriter, r *http.Request) (string, *model.User, *model.UserSettings, bool) {
	userID, err := h.sessionMgr.Validate(r.Context(), r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return "", nil, nil, false
	}
	user, _ := h.users.GetByID(r.Context(), userID)
	settings, _ := h.users.GetSettings(r.Context(), userID)
	return userID, user, settings, true
}

func (h *PageHandler) requireAdmin(w http.ResponseWriter, r *http.Request) (string, *model.User, *model.UserSettings, bool) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return "", nil, nil, false
	}
	if user == nil || user.Role != model.UserRoleAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return "", nil, nil, false
	}
	return userID, user, settings, true
}

func (h *PageHandler) baseData(pageName string, user *model.User, settings *model.UserSettings) map[string]any {
	theme := "oled"
	lang := "en"
	if settings != nil {
		theme = settings.Theme
		lang = settings.Locale
	}
	isAdmin := user != nil && user.Role == model.UserRoleAdmin
	return map[string]any{
		"Page":    pageName,
		"Theme":   theme,
		"Lang":    lang,
		"IsAdmin": isAdmin,
	}
}

func (h *PageHandler) region(settings *model.UserSettings) string {
	if settings != nil && settings.Region != "" {
		return settings.Region
	}
	return h.defaultRegion
}

// readAddStub pulls the optional stub_* form fields (populated by the
// add-to-library buttons on search / discover / preview / recommendation
// cards) into a model.AddStub. Returns nil if every field is empty so the
// service layer can distinguish "no stub provided" from "all-empty stub".
func readAddStub(r *http.Request) *model.AddStub {
	s := &model.AddStub{
		Title:       r.FormValue("stub_title"),
		Overview:    r.FormValue("stub_overview"),
		PosterPath:  r.FormValue("stub_poster"),
		ReleaseDate: r.FormValue("stub_release_date"),
	}
	if s.Title == "" && s.Overview == "" && s.PosterPath == "" && s.ReleaseDate == "" {
		return nil
	}
	return s
}

// --- Auth pages ---

func (h *PageHandler) Login(w http.ResponseWriter, r *http.Request) {
	if _, err := h.sessionMgr.Validate(r.Context(), r); err == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	h.render(w, "login", r, map[string]any{
		"Mode":             "login",
		"RegistrationOpen": !h.disableRegistration,
	})
}

func (h *PageHandler) Register(w http.ResponseWriter, r *http.Request) {
	if _, err := h.sessionMgr.Validate(r.Context(), r); err == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	h.render(w, "login", r, map[string]any{"Mode": "register"})
}

func (h *PageHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	user, err := h.users.Login(r.Context(), username, password)
	if err != nil {
		h.render(w, "login", r, map[string]any{
			"Mode":             "login",
			"Error":            "Invalid username or password",
			"Username":         username,
			"RegistrationOpen": !h.disableRegistration,
		})
		return
	}
	if _, err := h.sessionMgr.Create(r.Context(), w, user.ID); err != nil {
		h.render(w, "login", r, map[string]any{
			"Mode":             "login",
			"Error":            "Failed to create session",
			"RegistrationOpen": !h.disableRegistration,
		})
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *PageHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password_confirm")
	if password != passwordConfirm {
		h.render(w, "login", r, map[string]any{
			"Mode":     "register",
			"Error":    "Passwords do not match",
			"Username": username,
		})
		return
	}
	user := model.User{Username: username, Password: password}
	if err := h.users.Register(r.Context(), &user); err != nil {
		h.render(w, "login", r, map[string]any{
			"Mode":     "register",
			"Error":    err.Error(),
			"Username": username,
		})
		return
	}
	loggedIn, err := h.users.Login(r.Context(), username, password)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if _, err := h.sessionMgr.Create(r.Context(), w, loggedIn.ID); err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *PageHandler) Logout(w http.ResponseWriter, r *http.Request) {
	_, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	h.render(w, "logout", r, h.baseData("logout", user, settings))
}

func (h *PageHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	_ = h.sessionMgr.Destroy(r.Context(), w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// --- Library ---

func (h *PageHandler) Library(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	q := r.URL.Query()
	view := q.Get("view")

	data := h.baseData("library", user, settings)

	if view == "all" {
		filter := model.LibraryFilter{
			Status:    model.MediaStatus(q.Get("status")),
			MediaType: model.MediaType(q.Get("type")),
			TagID:     q.Get("tag"),
			SortBy:    q.Get("sort"),
			SortDir:   q.Get("dir"),
		}
		page := model.PageRequest{PageSize: 40, PageToken: q.Get("page_token")}
		result, err := h.libraryRepo.List(r.Context(), userID, filter, page)
		if err != nil {
			slog.Error("listing library", "error", err)
			result = &model.PageResponse[model.LibraryView]{}
		}
		result.EnsureItems()

		items := toCards(result.Items)
		data["View"] = "all"
		data["Items"] = items
		data["NextPageToken"] = result.NextPageToken
		data["TotalSize"] = result.TotalSize
		data["Filter"] = filter
		data["AutoRefresh"] = anyRefreshing(items)
	} else {
		stats, _ := h.stats.GetUserStats(r.Context(), userID)
		upcoming, _ := h.calendar.Upcoming(r.Context(), userID, model.CalendarFilter{Range: "30d"})
		if len(upcoming) > 5 {
			upcoming = upcoming[:5]
		}
		watching, _ := h.libraryRepo.ListContinueWatching(r.Context(), userID, 10)
		unrated, _ := h.libraryRepo.ListUnratedWatched(r.Context(), userID, 10)
		totalSize, _ := h.libraryRepo.TotalCount(r.Context(), userID)

		watchingCards := toCards(watching)
		unratedCards := toCards(unrated)
		data["View"] = ""
		data["Stats"] = stats
		data["Upcoming"] = upcoming
		data["Watching"] = watchingCards
		data["Unrated"] = unratedCards
		data["AutoRefresh"] = anyRefreshing(watchingCards) || anyRefreshing(unratedCards)
		data["TotalSize"] = totalSize
	}

	h.render(w, "library", r, data)
}

// --- Media detail ---

func (h *PageHandler) MediaDetail(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	libraryID := chi.URLParam(r, "id")
	v, err := h.libraryRepo.GetByID(r.Context(), libraryID)
	if err != nil || v == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	card := toCard(*v)
	data := h.baseData("library", user, settings)
	data["Media"] = card
	data["AutoRefresh"] = card.Refreshing

	// Rating: lookup by subject.
	var rating int
	if v.Show != nil {
		if r, err := h.ratingRepo.GetShow(r.Context(), userID, v.Show.ID); err == nil && r != nil {
			rating = r.Score
		}
	} else if v.Movie != nil {
		if r, err := h.ratingRepo.GetMovie(r.Context(), userID, v.Movie.ID); err == nil && r != nil {
			rating = r.Score
		}
	}
	data["Rating"] = rating

	// Tags + all user tags.
	mediaTags, _ := h.libraryTag.ListByLibrary(r.Context(), v.Entry.ID)
	allTags, _ := h.tags.List(r.Context(), userID)
	data["MediaTags"] = mediaTags
	data["AllTags"] = allTags

	// Watch providers + recommendations.
	region := h.region(settings)
	providers, _ := h.discovery.WatchProviders(r.Context(), v.Entry.ID, region)
	data["Providers"] = providers
	recs, _ := h.discovery.MediaRecommendations(r.Context(), v.Entry.ID)
	data["Recommendations"] = h.enrichRecommendations(r.Context(), userID, recs)

	if v.Entry.MediaType == model.MediaTypeTV && v.Show != nil {
		seasons, _ := h.seasons.ListByShow(r.Context(), v.Show.ID)
		allEps, _ := h.episodes.ListByShow(r.Context(), v.Show.ID)
		watchedMap, _ := h.events.WatchedMapForShow(r.Context(), userID, v.Show.ID)
		epScores, _ := h.ratingRepo.ListEpisodeRatingsByShow(r.Context(), userID, v.Show.ID)
		seasonScores, _ := h.ratingRepo.ListSeasonRatingsByShow(r.Context(), userID, v.Show.ID)

		type EpisodeRow struct {
			ID             string
			EpisodeNumber  int
			Name           string
			Overview       string
			AirDate        *int64
			RuntimeMinutes int
			Watched        bool
			WatchedAt      *int64
			UserRating     *int
		}
		type SeasonShape struct {
			ID           string
			SeasonNumber int
			Name         string
			WatchedCount int
			EpisodeCount int
			UserRating   *int
		}
		type SeasonWithEpisodes struct {
			Season   SeasonShape
			Episodes []EpisodeRow
		}

		episodesBySeason := map[string][]EpisodeRow{}
		for _, e := range allEps {
			row := EpisodeRow{
				ID:             e.ID,
				EpisodeNumber:  e.EpisodeNumber,
				Name:           e.Name,
				Overview:       e.Overview,
				AirDate:        e.AirDate,
				RuntimeMinutes: e.RuntimeMinutes,
			}
			if t, ok := watchedMap[e.ID]; ok {
				row.Watched = true
				tt := t
				row.WatchedAt = &tt
			}
			if score, ok := epScores[e.ID]; ok {
				s := score
				row.UserRating = &s
			}
			episodesBySeason[e.SeasonID] = append(episodesBySeason[e.SeasonID], row)
		}

		var seasonsData []SeasonWithEpisodes
		for _, s := range seasons {
			eps := episodesBySeason[s.ID]
			sh := SeasonShape{
				ID:           s.ID,
				SeasonNumber: s.SeasonNumber,
				Name:         s.Name,
				EpisodeCount: len(eps),
			}
			if score, ok := seasonScores[s.ID]; ok {
				sc := score
				sh.UserRating = &sc
			}
			for _, e := range eps {
				if e.Watched {
					sh.WatchedCount++
				}
			}
			seasonsData = append(seasonsData, SeasonWithEpisodes{Season: sh, Episodes: eps})
		}
		data["Seasons"] = seasonsData

		watched, total, _ := h.events.ProgressForShow(r.Context(), userID, v.Show.ID)
		data["WatchedCount"] = watched
		data["TotalEpisodes"] = total

		if next, err := h.events.NextUnwatched(r.Context(), userID, v.Show.ID); err == nil {
			data["NextEpisode"] = next
		}
	}

	data["Msg"] = r.URL.Query().Get("msg")
	h.render(w, "media_detail", r, data)
}

// --- Library item mutators ---

func (h *PageHandler) HandleSetStatus(w http.ResponseWriter, r *http.Request) {
	libraryID := chi.URLParam(r, "id")
	statusStr := r.FormValue("status")
	var status *model.MediaStatus
	if statusStr != "" {
		s := model.MediaStatus(statusStr)
		status = &s
	}
	if err := h.library.SetStatus(r.Context(), libraryID, status); err != nil {
		slog.Error("set status", "error", err)
	}
	http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
}

func (h *PageHandler) HandleRateMedia(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	libraryID := chi.URLParam(r, "id")
	score, _ := strconv.Atoi(r.FormValue("score"))

	v, err := h.libraryRepo.GetByID(r.Context(), libraryID)
	if err != nil || v == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	switch {
	case v.Show != nil:
		_ = h.ratings.RateShow(r.Context(), userID, v.Show.ID, score)
	case v.Movie != nil:
		_ = h.ratings.RateMovie(r.Context(), userID, v.Movie.ID, score)
	}
	http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
}

func (h *PageHandler) HandleRemoveMedia(w http.ResponseWriter, r *http.Request) {
	libraryID := chi.URLParam(r, "id")
	if err := h.library.Remove(r.Context(), libraryID); err != nil {
		slog.Error("remove media", "error", err)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *PageHandler) HandleAddTag(w http.ResponseWriter, r *http.Request) {
	libraryID := chi.URLParam(r, "id")
	tagID := r.FormValue("tag_id")
	if err := h.libraryTag.Add(r.Context(), libraryID, tagID); err != nil {
		slog.Error("add tag", "error", err)
	}
	http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
}

func (h *PageHandler) HandleRemoveTag(w http.ResponseWriter, r *http.Request) {
	libraryID := chi.URLParam(r, "id")
	tagID := chi.URLParam(r, "tagId")
	if err := h.libraryTag.Remove(r.Context(), libraryID, tagID); err != nil {
		slog.Error("remove tag", "error", err)
	}
	http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
}

func (h *PageHandler) HandleWatchNext(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	libraryID := chi.URLParam(r, "id")
	v, err := h.libraryRepo.GetByID(r.Context(), libraryID)
	if err == nil && v != nil && v.Show != nil {
		if _, err := h.watch.WatchNext(r.Context(), userID, v.Show.ID); err != nil {
			slog.Error("watch next", "error", err)
		}
	}
	http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
}

// --- Episode mutators ---

func (h *PageHandler) HandleWatchEpisode(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	episodeID := chi.URLParam(r, "id")
	if err := h.watch.WatchEpisode(r.Context(), userID, episodeID, 0, ""); err != nil {
		slog.Error("watch episode", "error", err)
	}
	h.redirectAfterEpisode(w, r, episodeID, r.FormValue("return_to"))
}

func (h *PageHandler) HandleUnwatchEpisode(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	episodeID := chi.URLParam(r, "id")
	if err := h.watch.UnwatchAllForEpisode(r.Context(), userID, episodeID); err != nil {
		slog.Error("unwatch episode", "error", err)
	}
	h.redirectAfterEpisode(w, r, episodeID, r.FormValue("return_to"))
}

func (h *PageHandler) HandleRateEpisode(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	episodeID := chi.URLParam(r, "id")
	score, _ := strconv.Atoi(r.FormValue("score"))
	_ = h.ratings.RateEpisode(r.Context(), userID, episodeID, score)
	h.redirectAfterEpisode(w, r, episodeID, r.FormValue("return_to"))
}

func (h *PageHandler) HandleWatchAllSeason(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	seasonID := chi.URLParam(r, "id")
	if err := h.watch.WatchAllInSeason(r.Context(), userID, seasonID); err != nil {
		slog.Error("watch all season", "error", err)
	}
	http.Redirect(w, r, r.FormValue("return_to"), http.StatusSeeOther)
}

func (h *PageHandler) HandleRateSeason(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	seasonID := chi.URLParam(r, "id")
	score, _ := strconv.Atoi(r.FormValue("score"))
	_ = h.ratings.RateSeason(r.Context(), userID, seasonID, score)
	http.Redirect(w, r, r.FormValue("return_to"), http.StatusSeeOther)
}

func (h *PageHandler) HandleWatchUpTo(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	episodeID := chi.URLParam(r, "id")
	if err := h.watch.WatchUpToEpisode(r.Context(), userID, episodeID); err != nil {
		slog.Error("watch up to", "error", err)
	}
	h.redirectAfterEpisode(w, r, episodeID, r.FormValue("return_to"))
}

func (h *PageHandler) HandleUnwatchUpTo(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	episodeID := chi.URLParam(r, "id")
	if err := h.watch.UnwatchUpToEpisode(r.Context(), userID, episodeID); err != nil {
		slog.Error("unwatch up to", "error", err)
	}
	h.redirectAfterEpisode(w, r, episodeID, r.FormValue("return_to"))
}

// HandleWatchUpToDate is the "mark up to here with a chosen date mode"
// affordance on the media detail page. Supported modes: "today" (default),
// "release" (each event stamped with its episode's air date), and "custom"
// (a single user-picked date applied to every inserted event).
func (h *PageHandler) HandleWatchUpToDate(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	libraryID := chi.URLParam(r, "id")
	episodeID := r.FormValue("season_episode")
	if episodeID == "" {
		http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
		return
	}
	mode := r.FormValue("date_mode")
	var customAt int64
	if mode == "custom" {
		if t, err := time.Parse("2006-01-02", r.FormValue("custom_date")); err == nil {
			customAt = t.UTC().Unix()
		} else {
			// Fall back to today when the user picked "custom" but the
			// date field was blank or malformed.
			mode = "today"
		}
	}
	if err := h.watch.WatchUpToEpisodeWithDate(r.Context(), userID, episodeID, mode, customAt); err != nil {
		slog.Error("watch up to date", "error", err)
	}
	http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
}

// --- Search ---

func (h *PageHandler) Search(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	data := h.baseData("search", user, settings)
	query := r.URL.Query().Get("q")
	searchType := r.URL.Query().Get("type")
	data["Query"] = query
	data["SearchType"] = searchType

	if query != "" {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		var results *tmdb.SearchResponse
		var err error
		switch searchType {
		case "movie":
			results, err = h.tmdbClient.SearchMovies(r.Context(), query, page)
		case "tv":
			results, err = h.tmdbClient.SearchTV(r.Context(), query, page)
		default:
			results, err = h.tmdbClient.SearchMulti(r.Context(), query, page)
		}
		if err != nil {
			slog.Error("search", "error", err)
		} else {
			var filtered []tmdb.SearchResult
			for _, sr := range results.Results {
				if sr.MediaType == "movie" || sr.MediaType == "tv" {
					filtered = append(filtered, sr)
				}
			}
			libMap, _ := h.libraryRepo.GetLibraryMap(r.Context(), userID)

			type SearchItem struct {
				tmdb.SearchResult
				InLibrary bool
				MediaID   string
			}
			var items []SearchItem
			for _, sr := range filtered {
				key := fmt.Sprintf("%d:%s", sr.ID, sr.MediaType)
				item := SearchItem{SearchResult: sr}
				if id, ok := libMap[key]; ok {
					item.InLibrary = true
					item.MediaID = id
				}
				items = append(items, item)
			}
			data["Results"] = items
			data["SearchPage"] = results.Page
			data["TotalPages"] = results.TotalPages
			data["TotalResults"] = results.TotalResults
		}
	}
	data["Msg"] = r.URL.Query().Get("msg")
	h.render(w, "search", r, data)
}

func (h *PageHandler) HandleSearchAdd(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	tmdbID, _ := strconv.Atoi(r.FormValue("tmdb_id"))
	mediaType := model.MediaType(r.FormValue("media_type"))
	query := r.FormValue("q")
	page := r.FormValue("page")

	if _, err := h.library.AddWithStub(r.Context(), userID, tmdbID, mediaType, readAddStub(r)); err != nil {
		slog.Error("add from search", "error", err)
	}
	redirectURL := "/search?q=" + url.QueryEscape(query)
	if page != "" && page != "1" {
		redirectURL += "&page=" + page
	}
	if st := r.FormValue("type"); st != "" {
		redirectURL += "&type=" + st
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (h *PageHandler) Preview(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	tmdbID, _ := strconv.Atoi(r.URL.Query().Get("tmdb_id"))
	mediaType := r.URL.Query().Get("type")
	query := r.URL.Query().Get("q")
	region := h.region(settings)

	// Cache only the TMDB-derived fields; per-user state (InLibrary,
	// MediaID, settings) is merged in below on every request so a
	// cached entry can't leak state across users.
	cacheKey := previewKey(tmdbID, mediaType, region)
	tmdbData, hit := h.previews.get(cacheKey)
	if !hit {
		var err error
		tmdbData, err = h.buildPreviewTMDBData(r.Context(), tmdbID, mediaType, region)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		h.previews.set(cacheKey, tmdbData)
	}

	data := h.baseData("search", user, settings)
	data["Query"] = query
	data["TMDBID"] = tmdbID
	data["MediaType"] = mediaType
	for k, v := range tmdbData {
		data[k] = v
	}

	libMap, _ := h.libraryRepo.GetLibraryMap(r.Context(), userID)
	key := fmt.Sprintf("%d:%s", tmdbID, mediaType)
	if id, ok := libMap[key]; ok {
		data["InLibrary"] = true
		data["MediaID"] = id
	}
	h.render(w, "preview", r, data)
}

// buildPreviewTMDBData fires every TMDB call the preview page needs and
// returns the derived template fields as a map. Does NOT include
// per-user state — that's merged in by Preview on every request so the
// cache stays user-agnostic.
func (h *PageHandler) buildPreviewTMDBData(ctx context.Context, tmdbID int, mediaType, region string) (map[string]any, error) {
	out := map[string]any{}

	if mediaType == "movie" {
		details, err := h.tmdbClient.GetMovie(ctx, tmdbID)
		if err != nil {
			return nil, err
		}
		out["Title"] = details.Title
		out["Overview"] = details.Overview
		out["PosterPath"] = details.PosterPath
		out["ReleaseDate"] = details.ReleaseDate
		out["Runtime"] = details.Runtime
		out["VoteAverage"] = details.VoteAverage
		genres := make([]string, len(details.Genres))
		for i, g := range details.Genres {
			genres[i] = g.Name
		}
		out["Genres"] = strings.Join(genres, ", ")

		if providers, err := h.tmdbClient.GetMovieWatchProviders(ctx, tmdbID, region); err == nil && providers != nil {
			out["Providers"] = providers
		}
		return out, nil
	}

	details, err := h.tmdbClient.GetTV(ctx, tmdbID)
	if err != nil {
		return nil, err
	}
	out["Title"] = details.Name
	out["Overview"] = details.Overview
	out["PosterPath"] = details.PosterPath
	out["ReleaseDate"] = details.FirstAirDate
	out["VoteAverage"] = details.VoteAverage
	genres := make([]string, len(details.Genres))
	for i, g := range details.Genres {
		genres[i] = g.Name
	}
	out["Genres"] = strings.Join(genres, ", ")

	// Per-season episode lists — fetched in parallel (bounded) so
	// even a 26-season show renders in ~one HTTP roundtrip worth
	// of latency. Read-only: the preview page doesn't have watch /
	// rate affordances since the item isn't in the library.
	type previewEpisode struct {
		EpisodeNumber  int
		Name           string
		AirDate        string
		Overview       string
		RuntimeMinutes int
	}
	type previewSeason struct {
		Name         string
		SeasonNumber int
		EpisodeCount int
		AirDate      string
		Episodes     []previewEpisode
	}

	seasonDetails := make([]*tmdb.SeasonDetails, len(details.Seasons))
	var (
		wg  sync.WaitGroup
		sem = make(chan struct{}, 6)
	)
	for i, ts := range details.Seasons {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx, sn int) {
			defer wg.Done()
			defer func() { <-sem }()
			sd, err := h.tmdbClient.GetSeason(ctx, tmdbID, sn)
			if err != nil {
				return
			}
			seasonDetails[idx] = sd
		}(i, ts.SeasonNumber)
	}
	wg.Wait()

	var seasons []previewSeason
	for i, ts := range details.Seasons {
		ps := previewSeason{
			Name:         ts.Name,
			SeasonNumber: ts.SeasonNumber,
			EpisodeCount: ts.EpisodeCount,
			AirDate:      ts.AirDate,
		}
		if seasonDetails[i] != nil {
			for _, e := range seasonDetails[i].Episodes {
				ps.Episodes = append(ps.Episodes, previewEpisode{
					EpisodeNumber:  e.EpisodeNumber,
					Name:           e.Name,
					AirDate:        e.AirDate,
					Overview:       e.Overview,
					RuntimeMinutes: e.Runtime,
				})
			}
		}
		seasons = append(seasons, ps)
	}
	out["PreviewSeasons"] = seasons

	if providers, err := h.tmdbClient.GetTVWatchProviders(ctx, tmdbID, region); err == nil && providers != nil {
		out["Providers"] = providers
	}
	return out, nil
}

func (h *PageHandler) HandleSearchAddAndView(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	tmdbID, _ := strconv.Atoi(r.FormValue("tmdb_id"))
	mediaType := model.MediaType(r.FormValue("media_type"))
	v, err := h.library.AddWithStub(r.Context(), userID, tmdbID, mediaType, readAddStub(r))
	if err != nil {
		existing, _ := h.libraryRepo.GetByTMDBID(r.Context(), userID, tmdbID, mediaType)
		if existing != nil {
			http.Redirect(w, r, "/media/"+existing.Entry.ID, http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/search", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/media/"+v.Entry.ID, http.StatusSeeOther)
}

// --- Calendar ---

func (h *PageHandler) Calendar(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	filter := model.CalendarFilter{
		MediaType: model.MediaType(q.Get("type")),
		Range:     q.Get("range"),
	}
	if filter.Range == "" {
		filter.Range = "7d"
	}
	upcoming, _ := h.calendar.Upcoming(r.Context(), userID, filter)
	recent, _ := h.calendar.RecentlyReleased(r.Context(), userID, filter)
	data := h.baseData("calendar", user, settings)
	data["Upcoming"] = upcoming
	data["Recent"] = recent
	data["Filter"] = filter
	data["Section"] = q.Get("section")
	h.render(w, "calendar", r, data)
}

// --- Discover ---

func (h *PageHandler) Discover(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	tab := q.Get("tab")
	if tab == "" {
		tab = "trending"
	}
	data := h.baseData("discover", user, settings)
	data["Tab"] = tab
	data["Msg"] = q.Get("msg")

	switch tab {
	case "trending":
		trendingType := q.Get("type")
		if trendingType == "" {
			trendingType = "movie"
		}
		result, err := h.discovery.Trending(r.Context())
		if err != nil {
			slog.Error("trending", "error", err)
		}
		if result != nil {
			var items []model.DiscoverItem
			if trendingType == "tv" {
				items = result.TV
			} else {
				items = result.Movies
			}
			data["TrendingItems"] = h.enrichDiscoverItems(r.Context(), userID, items)
		}
		data["TrendingType"] = trendingType
	case "popular":
		mediaType := model.MediaType(q.Get("type"))
		if mediaType == "" {
			mediaType = model.MediaTypeMovie
		}
		page, _ := strconv.Atoi(q.Get("page"))
		if page < 1 {
			page = 1
		}
		result, err := h.discovery.Popular(r.Context(), mediaType, page)
		if err != nil {
			slog.Error("popular", "error", err)
		}
		if result != nil {
			result.Items = h.enrichDiscoverItems(r.Context(), userID, result.Items)
		}
		data["Popular"] = result
		data["PopularType"] = mediaType
		data["PopularPage"] = page
	case "recommendations":
		recs, err := h.discovery.Recommendations(r.Context(), userID)
		if err != nil {
			slog.Error("recommendations", "error", err)
		}
		if len(recs) > 0 {
			data["NextRecommendation"] = recs[0]
			data["RemainingCount"] = len(recs) - 1
		}
	}
	h.render(w, "discover", r, data)
}

func (h *PageHandler) HandleDiscoverAdd(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	tmdbID, _ := strconv.Atoi(r.FormValue("tmdb_id"))
	mediaType := model.MediaType(r.FormValue("media_type"))
	returnTo := r.FormValue("return_to")
	if _, err := h.library.AddWithStub(r.Context(), userID, tmdbID, mediaType, readAddStub(r)); err != nil {
		slog.Error("add from discover", "error", err)
	}
	if returnTo == "" {
		returnTo = "/discover"
	}
	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (h *PageHandler) HandleDismissRecommendation(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	tmdbID, _ := strconv.Atoi(r.FormValue("tmdb_id"))
	mediaType := r.FormValue("media_type")
	if err := h.dismissedRepo.Dismiss(r.Context(), userID, tmdbID, mediaType); err != nil {
		slog.Error("dismiss recommendation", "error", err)
	}
	http.Redirect(w, r, "/discover?tab=recommendations", http.StatusSeeOther)
}

// --- Suggestions ---

func (h *PageHandler) Suggestions(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	suggestions, _ := h.keywords.ListSuggestions(r.Context(), userID)
	watches, _ := h.keywords.List(r.Context(), userID)
	showDismissed := r.URL.Query().Get("show") == "dismissed"
	var dismissed []model.KeywordResult
	if showDismissed {
		dismissed, _ = h.keywords.ListDismissed(r.Context(), userID)
	}
	kwNames := map[string]string{}
	for _, kw := range watches {
		kwNames[kw.ID] = kw.Keyword
	}
	data := h.baseData("suggestions", user, settings)
	data["Suggestions"] = suggestions
	data["Dismissed"] = dismissed
	data["ShowDismissed"] = showDismissed
	data["KeywordNames"] = kwNames
	h.render(w, "suggestions", r, data)
}

func (h *PageHandler) HandleSuggestionAdd(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	resultID := chi.URLParam(r, "id")
	tmdbID, _ := strconv.Atoi(r.FormValue("tmdb_id"))
	mediaType := model.MediaType(r.FormValue("media_type"))
	if tmdbID > 0 {
		if _, err := h.library.AddWithStub(r.Context(), userID, tmdbID, mediaType, readAddStub(r)); err != nil {
			slog.Error("add suggestion to library", "error", err)
		}
	}
	if err := h.keywords.AddSuggestion(r.Context(), resultID); err != nil {
		slog.Error("mark suggestion added", "error", err)
	}
	http.Redirect(w, r, "/suggestions", http.StatusSeeOther)
}

func (h *PageHandler) HandleSuggestionDismiss(w http.ResponseWriter, r *http.Request) {
	resultID := chi.URLParam(r, "id")
	_ = h.keywords.DismissSuggestion(r.Context(), resultID)
	http.Redirect(w, r, "/suggestions", http.StatusSeeOther)
}

func (h *PageHandler) HandleSuggestionDismissAll(w http.ResponseWriter, r *http.Request) {
	kwID := chi.URLParam(r, "kwId")
	_ = h.keywords.DismissAll(r.Context(), kwID)
	http.Redirect(w, r, "/suggestions", http.StatusSeeOther)
}

func (h *PageHandler) HandleSuggestionRestore(w http.ResponseWriter, r *http.Request) {
	resultID := chi.URLParam(r, "id")
	_ = h.keywords.RestoreSuggestion(r.Context(), resultID)
	http.Redirect(w, r, "/suggestions?show=dismissed", http.StatusSeeOther)
}

// --- Keywords ---

func (h *PageHandler) Keywords(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	watches, _ := h.keywords.List(r.Context(), userID)
	data := h.baseData("keywords", user, settings)
	data["Watches"] = watches
	data["Msg"] = r.URL.Query().Get("msg")
	h.render(w, "keywords", r, data)
}

func (h *PageHandler) HandleKeywordCreate(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	keyword := r.FormValue("keyword")
	mediaTypes := r.FormValue("media_types")
	if _, err := h.keywords.Create(r.Context(), userID, keyword, mediaTypes); err != nil {
		slog.Error("create keyword", "error", err)
	}
	http.Redirect(w, r, "/keywords", http.StatusSeeOther)
}

func (h *PageHandler) HandleKeywordUpdate(w http.ResponseWriter, r *http.Request) {
	kwID := chi.URLParam(r, "id")
	kw := &model.KeywordWatch{
		Keyword:    r.FormValue("keyword"),
		MediaTypes: r.FormValue("media_types"),
	}
	_ = h.keywords.Update(r.Context(), kwID, []string{"keyword", "media_types"}, kw)
	http.Redirect(w, r, "/keywords", http.StatusSeeOther)
}

func (h *PageHandler) HandleKeywordDelete(w http.ResponseWriter, r *http.Request) {
	kwID := chi.URLParam(r, "id")
	_ = h.keywords.Delete(r.Context(), kwID)
	http.Redirect(w, r, "/keywords", http.StatusSeeOther)
}

// --- Settings ---

func (h *PageHandler) Settings(w http.ResponseWriter, r *http.Request) {
	_, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	data := h.baseData("settings", user, settings)
	data["Settings"] = settings
	if settings != nil {
		data["ICSToken"] = settings.ICSToken
	}
	data["BaseURL"] = h.baseURL
	data["Msg"] = r.URL.Query().Get("msg")
	q := r.URL.Query()
	if q.Get("added") != "" {
		data["TraktAdded"] = q.Get("added")
		data["TraktSkipped"] = q.Get("skipped")
	}
	h.render(w, "settings", r, data)
}

func (h *PageHandler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	s := &model.UserSettings{
		UserID: userID,
		Theme:  r.FormValue("theme"),
		Locale: r.FormValue("locale"),
		Region: r.FormValue("region"),
	}
	_ = h.users.UpdateSettings(r.Context(), s, []string{"theme", "locale", "region"})
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (h *PageHandler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	current := r.FormValue("current_password")
	newPw := r.FormValue("new_password")
	confirmPw := r.FormValue("confirm_password")
	if newPw != confirmPw {
		http.Redirect(w, r, "/settings?msg=password_mismatch", http.StatusSeeOther)
		return
	}
	if err := h.users.ChangePassword(r.Context(), userID, current, newPw); err != nil {
		http.Redirect(w, r, "/settings?msg=password_error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// --- Library export / import ---

func (h *PageHandler) HandleLibraryExport(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	exporter := service.NewLibraryExporter(
		h.libraryRepo, h.libraryTag, h.events, h.ratingRepo,
		h.seasons, h.episodes, h.tagRepo,
	)
	data, err := exporter.Export(r.Context(), userID)
	if err != nil {
		slog.Error("library export", "error", err)
		http.Error(w, "export failed", http.StatusInternalServerError)
		return
	}
	filename := fmt.Sprintf("binge-library-%s.json", time.Now().UTC().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		slog.Error("encoding library export", "error", err)
	}
}

// HandleLibraryImport validates the JSON synchronously, persists it as a
// library_import_job row, and kicks off background processing. The HTTP
// request returns immediately so nginx / chi timeouts can't kill the
// import mid-run, and a server crash leaves the job row in place so the
// startup recovery can pick it back up.
func (h *PageHandler) HandleLibraryImport(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Redirect(w, r, "/settings?msg=import_error", http.StatusSeeOther)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Redirect(w, r, "/settings?msg=import_error", http.StatusSeeOther)
		return
	}
	defer file.Close()

	var data service.LibraryExport
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		slog.Error("library import decode", "error", err)
		http.Redirect(w, r, "/settings?msg=import_error", http.StatusSeeOther)
		return
	}

	jobID, err := h.importRunner.Enqueue(r.Context(), userID, &data)
	if err != nil {
		slog.Error("library import enqueue", "error", err)
		http.Redirect(w, r, "/settings?msg=import_error", http.StatusSeeOther)
		return
	}
	slog.Info("library import enqueued",
		"job_id", jobID, "user_id", userID,
		"movies", len(data.Movies), "shows", len(data.Shows), "tags", len(data.Tags))

	http.Redirect(w, r, "/settings?msg=import_started", http.StatusSeeOther)
}

// --- Admin ---

func (h *PageHandler) Admin(w http.ResponseWriter, r *http.Request) {
	_, user, settings, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	tab := q.Get("tab")
	if tab == "" {
		tab = "users"
	}
	data := h.baseData("admin", user, settings)
	data["Tab"] = tab
	data["CurrentUserID"] = user.ID

	if tab == "tmdb_jobs" {
		var jobs []model.TMDBJob
		if h.tmdbJobRepo != nil {
			jobs, _ = h.tmdbJobRepo.ListAll(r.Context())
		}
		// Counts by status so the tab header can show at-a-glance
		// queue depth without the reader eyeballing the list.
		var pending, running, failed int
		for _, j := range jobs {
			switch j.Status {
			case "pending":
				pending++
			case "running":
				running++
			case "failed":
				failed++
			}
		}
		data["TMDBJobs"] = jobs
		data["TMDBJobsPending"] = pending
		data["TMDBJobsRunning"] = running
		data["TMDBJobsFailed"] = failed
	} else if tab == "stats" {
		globalStats, err := h.stats.GetGlobalStats(r.Context())
		if err != nil {
			globalStats = &model.UserStats{}
		}
		adminStats, _ := h.stats.GetAdminStats(r.Context())
		if adminStats == nil {
			adminStats = &model.AdminStats{}
		}
		data["GlobalStats"] = globalStats
		data["AdminStats"] = adminStats
	} else {
		users, _ := h.users.ListAll(r.Context())
		roleFilter := q.Get("filter")
		if roleFilter != "" {
			var filtered []model.User
			for _, u := range users {
				if string(u.Role) == roleFilter {
					filtered = append(filtered, u)
				}
			}
			users = filtered
		}
		lastActive, _ := h.stats.GetLastActiveByUser(r.Context())
		data["Users"] = users
		data["Filter"] = roleFilter
		data["LastActive"] = lastActive
	}
	h.render(w, "admin", r, data)
}

func (h *PageHandler) HandleSetRole(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	targetID := r.FormValue("user_id")
	role := model.UserRole(r.FormValue("role"))
	_ = h.users.SetRole(r.Context(), userID, targetID, role)
	http.Redirect(w, r, "/admin?tab=users", http.StatusSeeOther)
}

func (h *PageHandler) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	targetID := r.FormValue("user_id")
	_ = h.users.DeleteUser(r.Context(), userID, targetID)
	http.Redirect(w, r, "/admin?tab=users", http.StatusSeeOther)
}

// HandleTMDBJobRetry re-runs a single tmdb_job regardless of current
// status. Admin-only affordance on the tmdb_jobs tab. The runner resets
// the row to running internally, so we only need to look it up and kick
// it via runJob (exposed as RetryOne).
func (h *PageHandler) HandleTMDBJobRetry(w http.ResponseWriter, r *http.Request) {
	if _, _, _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if h.tmdbJobRepo == nil || h.tmdbJobRunner == nil {
		http.Redirect(w, r, "/admin?tab=tmdb_jobs", http.StatusSeeOther)
		return
	}
	jobID := r.FormValue("job_id")
	if jobID != "" {
		// Find the job in the current list (ListAll is cheap; no
		// dedicated GetByID on this repo by design).
		jobs, _ := h.tmdbJobRepo.ListAll(r.Context())
		for _, j := range jobs {
			if j.ID == jobID {
				h.tmdbJobRunner.RetryOne(j)
				break
			}
		}
	}
	http.Redirect(w, r, "/admin?tab=tmdb_jobs", http.StatusSeeOther)
}

// HandleTMDBJobDelete removes a tmdb_job row outright. Used for
// permanently-unfixable failures (e.g. a TMDB id that no longer exists).
func (h *PageHandler) HandleTMDBJobDelete(w http.ResponseWriter, r *http.Request) {
	if _, _, _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if h.tmdbJobRepo == nil {
		http.Redirect(w, r, "/admin?tab=tmdb_jobs", http.StatusSeeOther)
		return
	}
	jobID := r.FormValue("job_id")
	if jobID != "" {
		_ = h.tmdbJobRepo.Delete(r.Context(), jobID)
	}
	http.Redirect(w, r, "/admin?tab=tmdb_jobs", http.StatusSeeOther)
}

// --- Webhooks ---

func (h *PageHandler) Webhooks(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	webhooks, _ := h.webhookRepo.ListByUser(r.Context(), userID)
	data := h.baseData("webhooks", user, settings)
	data["Webhooks"] = webhooks
	data["Msg"] = r.URL.Query().Get("msg")
	h.render(w, "webhooks", r, data)
}

func (h *PageHandler) WebhookDetail(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	whID := chi.URLParam(r, "id")
	webhooks, _ := h.webhookRepo.ListByUser(r.Context(), userID)
	var target *model.Webhook
	for i := range webhooks {
		if webhooks[i].ID == whID {
			target = &webhooks[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	deliveries, _ := h.webhookRepo.ListDeliveries(r.Context(), whID, 50)
	data := h.baseData("webhooks", user, settings)
	data["Webhook"] = target
	data["Deliveries"] = deliveries
	h.render(w, "webhook_detail", r, data)
}

func (h *PageHandler) WebhookEdit(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	whID := chi.URLParam(r, "id")
	webhooks, _ := h.webhookRepo.ListByUser(r.Context(), userID)
	var target *model.Webhook
	for i := range webhooks {
		if webhooks[i].ID == whID {
			target = &webhooks[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	events := strings.Split(target.Events, ",")
	hasEvent := func(e string) bool {
		for _, ev := range events {
			if strings.TrimSpace(ev) == e {
				return true
			}
		}
		return false
	}
	data := h.baseData("webhooks", user, settings)
	data["Webhook"] = target
	data["Msg"] = r.URL.Query().Get("msg")
	data["HasAdded"] = hasEvent(model.WebhookEventAdded)
	data["HasStatusChanged"] = hasEvent(model.WebhookEventStatusChanged)
	data["HasWatched"] = hasEvent(model.WebhookEventWatched)
	data["HasEpisodeWatched"] = hasEvent(model.WebhookEventEpisodeWatched)
	data["HasReleased"] = hasEvent(model.WebhookEventReleased)
	h.render(w, "webhook_edit", r, data)
}

func (h *PageHandler) HandleWebhookUpdate(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	whID := chi.URLParam(r, "id")
	webhooks, _ := h.webhookRepo.ListByUser(r.Context(), userID)
	var found bool
	for _, wh := range webhooks {
		if wh.ID == whID {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	rawURL := r.FormValue("url")
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		http.Redirect(w, r, "/webhooks/"+whID+"/edit?msg=webhook_error", http.StatusSeeOther)
		return
	}
	svc := r.FormValue("service")
	if svc == "" {
		svc = model.WebhookServiceGeneric
	}
	r.ParseForm()
	events := strings.Join(r.Form["events"], ",")
	if events == "" {
		events = model.WebhookEventStatusChanged
	}
	wh := &model.Webhook{
		ID:           whID,
		Name:         r.FormValue("name"),
		URL:          rawURL,
		Events:       events,
		Service:      svc,
		BodyTemplate: r.FormValue("body_template"),
		Headers:      r.FormValue("headers"),
	}
	if err := h.webhookRepo.Update(r.Context(), wh); err != nil {
		http.Redirect(w, r, "/webhooks/"+whID+"/edit?msg=webhook_error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/webhooks/"+whID, http.StatusSeeOther)
}

func (h *PageHandler) HandleWebhookCreate(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	rawURL := r.FormValue("url")
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		http.Redirect(w, r, "/webhooks?msg=webhook_error", http.StatusSeeOther)
		return
	}
	svc := r.FormValue("service")
	if svc == "" {
		svc = model.WebhookServiceGeneric
	}
	r.ParseForm()
	events := strings.Join(r.Form["events"], ",")
	if events == "" {
		events = model.WebhookEventStatusChanged
	}
	wh := &model.Webhook{
		UserID:       userID,
		Name:         r.FormValue("name"),
		URL:          rawURL,
		Events:       events,
		Service:      svc,
		BodyTemplate: r.FormValue("body_template"),
		Headers:      r.FormValue("headers"),
	}
	if err := h.webhookRepo.Create(r.Context(), wh); err != nil {
		http.Redirect(w, r, "/webhooks?msg=webhook_error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/webhooks", http.StatusSeeOther)
}

func (h *PageHandler) HandleWebhookDelete(w http.ResponseWriter, r *http.Request) {
	_, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	whID := chi.URLParam(r, "id")
	_ = h.webhookRepo.Delete(r.Context(), whID)
	http.Redirect(w, r, "/webhooks", http.StatusSeeOther)
}

func (h *PageHandler) HandleWebhookTest(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	whID := chi.URLParam(r, "id")
	webhooks, _ := h.webhookRepo.ListByUser(r.Context(), userID)
	for _, wh := range webhooks {
		if wh.ID == whID {
			go h.webhookSvc.DispatchTest(context.Background(), wh)
			break
		}
	}
	http.Redirect(w, r, "/webhooks/"+whID, http.StatusSeeOther)
}

// --- ICS ---

func (h *PageHandler) HandleICSFeed(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	user, err := h.users.GetByICSToken(r.Context(), token)
	if err != nil || user == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	feed, err := h.icsSvc.GenerateFeed(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Write([]byte(feed))
}

func (h *PageHandler) HandleRegenerateICS(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	if _, err := h.users.RegenerateICSToken(r.Context(), userID); err != nil {
		http.Redirect(w, r, "/settings?msg=ics_error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// --- Trakt import ---

func (h *PageHandler) HandleTraktImport(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Redirect(w, r, "/settings?msg=trakt_error", http.StatusSeeOther)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		http.Redirect(w, r, "/settings?msg=trakt_error", http.StatusSeeOther)
		return
	}
	added, skipped, err := h.traktSvc.Import(r.Context(), userID, data)
	if err != nil {
		http.Redirect(w, r, "/settings?msg=trakt_error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/settings?added=%d&skipped=%d", added, skipped), http.StatusSeeOther)
}

// --- Watched ---

func (h *PageHandler) Watched(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	mediaType := q.Get("type")
	sortBy := q.Get("sort")
	if sortBy == "" {
		sortBy = "watched_at"
	}
	filter := model.LibraryFilter{
		Statuses: []model.MediaStatus{model.MediaStatusWatched, model.MediaStatusWatching},
		SortBy:   sortBy,
	}
	if mediaType != "" {
		filter.MediaType = model.MediaType(mediaType)
	}
	page := model.PageRequest{PageSize: 50, PageToken: q.Get("page_token")}
	result, err := h.libraryRepo.List(r.Context(), userID, filter, page)
	if err != nil {
		slog.Error("listing watched", "error", err)
		result = &model.PageResponse[model.LibraryView]{}
	}
	result.EnsureItems()

	// Flatten views to cards so the template stays simple, and attach
	// ratings via the per-subject repo.
	type WatchedItem struct {
		Media  LibraryCard
		Rating int
	}
	var showIDs, movieIDs []string
	for _, v := range result.Items {
		if v.Show != nil {
			showIDs = append(showIDs, v.Show.ID)
		}
		if v.Movie != nil {
			movieIDs = append(movieIDs, v.Movie.ID)
		}
	}
	showRatings, _ := h.ratingRepo.ListShowRatingsByIDs(r.Context(), userID, showIDs)
	movieRatings, _ := h.ratingRepo.ListMovieRatingsByIDs(r.Context(), userID, movieIDs)

	items := make([]WatchedItem, len(result.Items))
	for i, v := range result.Items {
		card := toCard(v)
		var rating int
		if v.Show != nil {
			rating = showRatings[v.Show.ID]
		} else if v.Movie != nil {
			rating = movieRatings[v.Movie.ID]
		}
		items[i] = WatchedItem{Media: card, Rating: rating}
	}
	if sortBy == "rating" {
		for i := 0; i < len(items); i++ {
			for j := i + 1; j < len(items); j++ {
				if items[j].Rating > items[i].Rating {
					items[i], items[j] = items[j], items[i]
				}
			}
		}
	}
	data := h.baseData("watched", user, settings)
	data["Items"] = items
	data["TotalSize"] = result.TotalSize
	data["NextPageToken"] = result.NextPageToken
	data["MediaType"] = mediaType
	data["Sort"] = sortBy
	h.render(w, "watched", r, data)
}

// --- Episode detail ---

func (h *PageHandler) EpisodeDetail(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	episodeID := chi.URLParam(r, "id")
	ep, err := h.episodes.GetByID(r.Context(), episodeID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	watched, _ := h.events.HasEpisode(r.Context(), userID, episodeID)
	latest, _ := h.events.LatestForEpisode(r.Context(), userID, episodeID)
	season, _ := h.seasons.GetByID(r.Context(), ep.SeasonID)

	type EpisodeView struct {
		ID             string
		EpisodeNumber  int
		Name           string
		Overview       string
		StillPath      string
		AirDate        *int64
		RuntimeMinutes int
		Watched        bool
		WatchedAt      *int64
		Notes          string
		UserRating     int
	}
	ev := EpisodeView{
		ID:             ep.ID,
		EpisodeNumber:  ep.EpisodeNumber,
		Name:           ep.Name,
		Overview:       ep.Overview,
		StillPath:      ep.StillPath,
		AirDate:        ep.AirDate,
		RuntimeMinutes: ep.RuntimeMinutes,
		Watched:        watched,
	}
	if latest != nil {
		v := latest.WatchedAt
		ev.WatchedAt = &v
		ev.Notes = latest.Notes
	}
	if rating, _ := h.ratingRepo.GetEpisode(r.Context(), userID, episodeID); rating != nil {
		ev.UserRating = rating.Score
	}

	var seasonNumber int
	var libraryID string
	var mediaTitle string
	if season != nil {
		seasonNumber = season.SeasonNumber
		if lv, err := h.libraryRepo.GetByShow(r.Context(), userID, season.ShowID); err == nil && lv != nil {
			libraryID = lv.Entry.ID
			if lv.Show != nil {
				mediaTitle = lv.Show.Title
			}
		}
	}
	data := h.baseData("library", user, settings)
	data["Episode"] = ev
	data["SeasonNumber"] = seasonNumber
	data["MediaID"] = libraryID
	data["MediaTitle"] = mediaTitle
	h.render(w, "episode", r, data)
}

func (h *PageHandler) HandleEpisodeNotes(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	episodeID := chi.URLParam(r, "id")
	notes := r.FormValue("notes")
	// Notes live on the watch_event row under Option B. Try to update the
	// most recent event first; if the user has never marked the episode
	// watched, stamp one at "now" so the note has somewhere to live.
	updated, err := h.events.UpdateLatestNotesForEpisode(r.Context(), userID, episodeID, notes)
	if err != nil {
		slog.Error("update episode notes", "error", err)
	} else if !updated {
		if err := h.watch.WatchEpisode(r.Context(), userID, episodeID, 0, notes); err != nil {
			slog.Error("create episode watch for notes", "error", err)
		}
	}
	http.Redirect(w, r, "/episodes/"+episodeID, http.StatusSeeOther)
}

func (h *PageHandler) HandleEpisodeWatchedAt(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	episodeID := chi.URLParam(r, "id")
	dateStr := r.FormValue("watched_at")
	if dateStr != "" {
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			_ = h.watch.WatchEpisode(r.Context(), userID, episodeID, t.UTC().Unix(), "")
		}
	}
	http.Redirect(w, r, "/episodes/"+episodeID, http.StatusSeeOther)
}

func (h *PageHandler) redirectAfterEpisode(w http.ResponseWriter, r *http.Request, episodeID, returnTo string) {
	if returnTo == "" {
		if ep, err := h.episodes.GetByID(r.Context(), episodeID); err == nil {
			if season, err := h.seasons.GetByID(r.Context(), ep.SeasonID); err == nil && season != nil {
				if lv, err := h.libraryRepo.GetByShow(r.Context(), middleware.UserIDFromContext(r.Context()), season.ShowID); err == nil && lv != nil {
					returnTo = "/media/" + lv.Entry.ID
				}
			}
		}
		if returnTo == "" {
			returnTo = "/"
		}
	}
	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}

// --- Menu + tags ---

func (h *PageHandler) Menu(w http.ResponseWriter, r *http.Request) {
	_, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	h.render(w, "menu", r, h.baseData("menu", user, settings))
}

func (h *PageHandler) TagsPage(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	allTags, _ := h.tags.List(r.Context(), userID)
	data := h.baseData("tags", user, settings)
	data["AllTags"] = allTags
	h.render(w, "tags", r, data)
}

func (h *PageHandler) HandleTagPageCreate(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	name := r.FormValue("name")
	_, _ = h.tags.Create(r.Context(), userID, name)
	http.Redirect(w, r, "/tags", http.StatusSeeOther)
}

func (h *PageHandler) HandleTagPageDelete(w http.ResponseWriter, r *http.Request) {
	tagID := chi.URLParam(r, "id")
	_ = h.tags.Delete(r.Context(), tagID)
	http.Redirect(w, r, "/tags", http.StatusSeeOther)
}

// Legacy tag-create/delete via /settings — keep handlers so routes still compile.
func (h *PageHandler) HandleTagCreate(w http.ResponseWriter, r *http.Request) {
	h.HandleTagPageCreate(w, r)
}
func (h *PageHandler) HandleTagDelete(w http.ResponseWriter, r *http.Request) {
	h.HandleTagPageDelete(w, r)
}

// --- Library-level notes + watched date ---

func (h *PageHandler) HandleWatchedAt(w http.ResponseWriter, r *http.Request) {
	_, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	libraryID := chi.URLParam(r, "id")
	dateStr := r.FormValue("watched_at")
	if dateStr != "" {
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			v := t.UTC().Unix()
			_ = h.library.UpdateWatchedAt(r.Context(), libraryID, &v)
			// Nudge movies to "watched" status when the user sets a date.
			if lv, err := h.libraryRepo.GetByID(r.Context(), libraryID); err == nil && lv != nil && lv.Entry.MediaType == model.MediaTypeMovie {
				watched := model.MediaStatusWatched
				_ = h.library.SetStatus(r.Context(), libraryID, &watched)
			}
		}
	} else {
		_ = h.library.UpdateWatchedAt(r.Context(), libraryID, nil)
	}
	http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
}

func (h *PageHandler) HandleNotes(w http.ResponseWriter, r *http.Request) {
	_, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	libraryID := chi.URLParam(r, "id")
	notes := r.FormValue("notes")
	_ = h.library.UpdateNotes(r.Context(), libraryID, notes)
	http.Redirect(w, r, "/media/"+libraryID, http.StatusSeeOther)
}

// --- Discovery enrichment helpers ---

func (h *PageHandler) enrichDiscoverItems(ctx context.Context, userID string, items []model.DiscoverItem) []model.DiscoverItem {
	libMap, _ := h.libraryRepo.GetLibraryMap(ctx, userID)
	for i := range items {
		key := fmt.Sprintf("%d:%s", items[i].TMDBID, items[i].MediaType)
		if id, ok := libMap[key]; ok {
			items[i].InLibrary = true
			items[i].MediaID = id
		}
	}
	return items
}

func (h *PageHandler) enrichRecommendations(ctx context.Context, userID string, items []model.RecommendationItem) []model.RecommendationItem {
	libMap, _ := h.libraryRepo.GetLibraryMap(ctx, userID)
	for i := range items {
		key := fmt.Sprintf("%d:%s", items[i].TMDBID, items[i].MediaType)
		if id, ok := libMap[key]; ok {
			items[i].InLibrary = true
			items[i].MediaID = id
		}
	}
	return items
}

// --- Render ---

func (h *PageHandler) render(w http.ResponseWriter, page string, r *http.Request, data any) {
	tmpl, ok := h.pages[page]
	if !ok {
		http.Error(w, "page not found", http.StatusNotFound)
		return
	}
	if m, ok := data.(map[string]any); ok {
		lang := i18n.LangEN
		if l, ok := m["Lang"].(string); ok && l != "" {
			lang = i18n.ParseLang(l)
		}
		m["T"] = func(key string) string { return i18n.T(lang, key) }
		m["CSRFToken"] = middleware.CSRFToken(r)
		m["InlineCSS"] = h.inlineCSS
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("rendering template", "page", page, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
