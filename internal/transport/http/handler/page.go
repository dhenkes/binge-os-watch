package handler

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

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
		"login":            parse("login"),
		"logout":           parse("logout"),
		"library":          parse("library"),
		"menu":             parse("menu"),
		"media_detail":     parse("media_detail"),
		"search":           parse("search"),
		"settings":         parse("settings"),
		"calendar":         parse("calendar"),
		"discover":         parse("discover"),
		"suggestions":      parse("suggestions"),
		"keywords":         parse("keywords"),
		"keyword_new":      parse("keyword_new"),
		"keyword_edit":     parse("keyword_edit"),
		"admin":            parse("admin"),
		"tags":             parse("tags"),
		"tag_new":          parse("tag_new"),
		"tag_edit":         parse("tag_edit"),
		"episode":          parse("episode"),
		"watched":          parse("watched"),
		"preview":          parse("preview"),
		"webhooks":         parse("webhooks"),
		"webhook_detail":   parse("webhook_detail"),
		"webhook_edit":     parse("webhook_edit"),
		"webhook_new_type": parse("webhook_new_type"),
		"webhook_new":      parse("webhook_new"),
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

// --- Shared types ---

// LibraryCard is a flattened shape for templates that render a library card.
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

func anyRefreshing(cards []LibraryCard) bool {
	for _, c := range cards {
		if c.Refreshing {
			return true
		}
	}
	return false
}

func viewStatus(v model.LibraryView) model.MediaStatus {
	if v.Entry.ManualStatus != nil {
		return *v.Entry.ManualStatus
	}
	return model.MediaStatusPlanToWatch
}

// --- Shared helpers ---

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
