package main

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"

	"github.com/dhenkes/binge-os-watch/internal/auth"
	"github.com/dhenkes/binge-os-watch/internal/config"
	"github.com/dhenkes/binge-os-watch/internal/jobs"
	"github.com/dhenkes/binge-os-watch/internal/repository"
	"github.com/dhenkes/binge-os-watch/internal/service"
	"github.com/dhenkes/binge-os-watch/internal/tmdb"
	"github.com/dhenkes/binge-os-watch/internal/transport/http/handler"
	"github.com/dhenkes/binge-os-watch/internal/transport/http/middleware"
	"github.com/dhenkes/binge-os-watch/web"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()
	slog.Info("binge-os-watch starting", "db", cfg.DB.Path)

	if cfg.TMDB.APIKey == "" {
		slog.Warn("TMDB API key not configured — search and metadata features will not work")
	}

	db, err := repository.NewSQLiteDB(cfg.DB.Path)
	if err != nil {
		slog.Error("opening database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	tmdbClient := tmdb.NewClient(cfg.TMDB.APIKey)

	// Repositories.
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	showRepo := repository.NewTMDBShowRepository(db)
	movieRepo := repository.NewTMDBMovieRepository(db)
	seasonRepo := repository.NewTMDBSeasonRepository(db)
	episodeRepo := repository.NewTMDBEpisodeRepository(db)
	libraryRepo := repository.NewLibraryRepository(db)
	libraryTagRepo := repository.NewLibraryTagRepository(db)
	watchEventRepo := repository.NewWatchEventRepository(db)
	ratingRepo := repository.NewRatingV2Repository(db)
	tagRepo := repository.NewTagRepository(db)
	calendarRepo := repository.NewCalendarRepository(db)
	keywordWatchRepo := repository.NewKeywordWatchRepository(db)
	statsRepo := repository.NewStatsRepository(db)
	webhookRepo := repository.NewWebhookRepository(db)
	dismissedRepo := repository.NewDismissedRecommendationRepository(db)
	importJobRepo := repository.NewLibraryImportJobRepository(db)
	tmdbJobRepo := repository.NewTMDBJobRepository(db)

	// Auth.
	hasher := auth.NewPasswordHasher(auth.Argon2idParams{
		Time:    cfg.Argon2.Time,
		Memory:  cfg.Argon2.Memory,
		Threads: cfg.Argon2.Threads,
		KeyLen:  cfg.Argon2.KeyLength,
		SaltLen: cfg.Argon2.SaltLen,
	})
	sessionMgr := auth.NewSessionManager(sessionRepo, cfg.SessionDuration(), cfg.Session.SecureCookie)

	// Services.
	txFunc := repository.NewTxFunc(db)
	userSvc := service.NewUserService(userRepo, hasher)
	librarySvc := service.NewLibraryService(txFunc, showRepo, movieRepo, seasonRepo, episodeRepo, libraryRepo, tmdbClient)
	watchSvc := service.NewWatchService(watchEventRepo, libraryRepo, seasonRepo, episodeRepo)
	ratingSvc := service.NewRatingServiceV2(ratingRepo)
	tagSvc := service.NewTagService(tagRepo)
	calendarSvc := service.NewCalendarService(calendarRepo)
	discoverySvc := service.NewDiscoveryService(tmdbClient, libraryRepo, dismissedRepo)
	keywordSvc := service.NewKeywordWatchService(keywordWatchRepo, librarySvc)
	imageSvc := service.NewImageService(cfg.Server.ImageCacheDir)
	statsSvc := service.NewStatsService(statsRepo)
	webhookSvc := service.NewWebhookService(webhookRepo)
	icsSvc := service.NewICSService(calendarSvc)
	traktSvc := service.NewTraktService(librarySvc)

	librarySvc.SetWebhookDispatcher(webhookSvc)
	watchSvc.SetWebhookDispatcher(webhookSvc)

	// TMDB background work: durable queue so the UI never waits on
	// TMDB. The runner holds a pointer back to librarySvc for the
	// actual fetch; librarySvc uses the runner to enqueue.
	tmdbJobRunner := service.NewTMDBJobRunner(tmdbJobRepo, librarySvc)
	librarySvc.SetTMDBJobRunner(tmdbJobRunner)
	tmdbJobRunner.ResumeAll(context.Background())
	// pageHandler admin surface is wired after NewPageHandler below.

	// Library import: durable JSON queue + background runner.
	importer := service.NewLibraryImporter(
		librarySvc, libraryRepo, watchSvc, ratingSvc, tagSvc,
		libraryTagRepo, tagRepo, seasonRepo, episodeRepo,
	)
	importRunner := service.NewLibraryImportRunner(importJobRepo, importer)
	// Resume any jobs left over from a crashed/restarted previous run.
	importRunner.ResumeAll(context.Background())

	// Handlers.
	authHandler := handler.NewAuthHandler(userSvc, sessionMgr)
	searchHandler := handler.NewSearchHandler(tmdbClient)
	mediaHandler := handler.NewMediaHandler(librarySvc, libraryRepo)
	episodeHandler := handler.NewEpisodeHandler(watchSvc)
	ratingHandler := handler.NewRatingHandler(ratingSvc, libraryRepo, episodeRepo, seasonRepo)
	tagHandler := handler.NewTagHandler(tagSvc, libraryTagRepo)
	settingsHandler := handler.NewSettingsHandler(userSvc)
	calendarHandler := handler.NewCalendarHandler(calendarSvc)
	discoverHandler := handler.NewDiscoverHandler(discoverySvc, userSvc)
	keywordHandler := handler.NewKeywordWatchHandler(keywordSvc)
	imageHandler := handler.NewImageHandler(imageSvc)

	staticFS, _ := fs.Sub(web.StaticFS, "static")
	pageHandler := handler.NewPageHandler(
		web.TemplateFS, staticFS,
		userSvc, sessionMgr,
		librarySvc, libraryRepo, watchSvc, watchEventRepo,
		showRepo, movieRepo, seasonRepo, episodeRepo,
		ratingSvc, ratingRepo,
		tagSvc, tagRepo, libraryTagRepo,
		calendarSvc, discoverySvc, keywordSvc,
		statsSvc, dismissedRepo, importRunner, webhookRepo, webhookSvc,
		icsSvc, traktSvc, tmdbClient,
		cfg.Server.DisableRegistration,
		cfg.Server.BaseURL,
		cfg.TMDB.DefaultRegion,
	)
	pageHandler.SetTMDBJobHandles(tmdbJobRepo, tmdbJobRunner)

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(middleware.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	authLimit := httprate.Limit(5, time.Minute, httprate.WithKeyFuncs(httprate.KeyByIP))
	if !cfg.Server.DisableRegistration {
		r.With(authLimit).Post("/api/v1/users:register", authHandler.Register)
	}
	r.With(authLimit).Post("/api/v1/users:login", authHandler.Login)
	r.Post("/api/v1/users:logout", authHandler.Logout)

	// Authenticated JSON API. Mirrors the OpenAPI spec at api/openapi.yaml.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(sessionMgr))

		r.Get("/api/v1/users/me", authHandler.Me)
		r.Post("/api/v1/users:change-password", authHandler.ChangePassword)

		r.Get("/api/v1/search", searchHandler.Search)

		// Library
		r.Get("/api/v1/media", mediaHandler.List)
		r.Post("/api/v1/media", mediaHandler.Add)
		r.Get("/api/v1/media/{id}", mediaHandler.Get)
		r.Post("/api/v1/media/{id}:set-status", mediaHandler.SetStatus)
		r.Delete("/api/v1/media/{id}", mediaHandler.Remove)
		r.Post("/api/v1/media/{id}:watch-next", episodeHandler.WatchNext)
		r.Post("/api/v1/media/{id}:rate", ratingHandler.RateMedia)
		r.Post("/api/v1/media/{id}/tags", tagHandler.AddToMedia)
		r.Delete("/api/v1/media/{id}/tags/{tagId}", tagHandler.RemoveFromMedia)

		// Episodes & seasons
		r.Post("/api/v1/episodes/{id}:watch", episodeHandler.Watch)
		r.Post("/api/v1/episodes/{id}:unwatch", episodeHandler.Unwatch)
		r.Post("/api/v1/episodes/{id}:rate", ratingHandler.RateEpisode)
		r.Post("/api/v1/seasons/{id}:watch-all", episodeHandler.WatchAllInSeason)
		r.Post("/api/v1/seasons/{id}:rate", ratingHandler.RateSeason)

		r.Get("/api/v1/tags", tagHandler.List)
		r.Post("/api/v1/tags", tagHandler.Create)
		r.Delete("/api/v1/tags/{id}", tagHandler.Delete)

		r.Get("/api/v1/calendar", calendarHandler.Calendar)

		r.Get("/api/v1/discover/trending", discoverHandler.Trending)
		r.Get("/api/v1/discover/popular", discoverHandler.Popular)
		r.Get("/api/v1/discover/recommendations", discoverHandler.Recommendations)
		r.Get("/api/v1/media/{id}/providers", discoverHandler.WatchProviders)
		r.Get("/api/v1/media/{id}/recommendations", discoverHandler.MediaRecommendations)

		r.Get("/api/v1/keyword-watches", keywordHandler.List)
		r.Post("/api/v1/keyword-watches", keywordHandler.Create)
		r.Patch("/api/v1/keyword-watches/{id}", keywordHandler.Update)
		r.Delete("/api/v1/keyword-watches/{id}", keywordHandler.Delete)
		r.Get("/api/v1/keyword-watches/suggestions", keywordHandler.Suggestions)
		r.Get("/api/v1/keyword-watches/count", keywordHandler.PendingCount)
		r.Post("/api/v1/keyword-watches/{id}:dismiss-all", keywordHandler.DismissAll)
		r.Post("/api/v1/keyword-results/{id}:add", keywordHandler.AddResult)
		r.Post("/api/v1/keyword-results/{id}:dismiss", keywordHandler.DismissResult)

		r.Get("/api/v1/images/*", imageHandler.Get)

		r.Get("/api/v1/settings", settingsHandler.Get)
		r.Patch("/api/v1/settings", settingsHandler.Update)
	})

	// Public ICS feed (token-authenticated, no session required).
	r.Get("/ics/{token}", pageHandler.HandleICSFeed)

	// Pages (server-rendered HTML).
	if !cfg.Server.DisableUI {
		r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

		r.Group(func(r chi.Router) {
			r.Use(middleware.Security)
			r.Use(middleware.CSRF)

			r.Get("/login", pageHandler.Login)
			r.With(authLimit).Post("/login", pageHandler.HandleLogin)
			if !cfg.Server.DisableRegistration {
				r.Get("/register", pageHandler.Register)
				r.With(authLimit).Post("/register", pageHandler.HandleRegister)
			}
			r.Get("/logout", pageHandler.Logout)
			r.Post("/logout", pageHandler.HandleLogout)

			r.Get("/", pageHandler.Library)
			r.Get("/menu", pageHandler.Menu)
			r.Get("/watched", pageHandler.Watched)

			r.Get("/media/{id}", pageHandler.MediaDetail)
			r.Post("/media/{id}/set-status", pageHandler.HandleSetStatus)
			r.Post("/media/{id}/rate", pageHandler.HandleRateMedia)
			r.Post("/media/{id}/remove", pageHandler.HandleRemoveMedia)
			r.Post("/media/{id}/tags/add", pageHandler.HandleAddTag)
			r.Post("/media/{id}/tags/{tagId}/remove", pageHandler.HandleRemoveTag)
			r.Post("/media/{id}/watch-next", pageHandler.HandleWatchNext)
			r.Post("/media/{id}/watched-at", pageHandler.HandleWatchedAt)
			r.Post("/media/{id}/watch-up-to-date", pageHandler.HandleWatchUpToDate)
			r.Post("/media/{id}/notes", pageHandler.HandleNotes)

			r.Get("/episodes/{id}", pageHandler.EpisodeDetail)
			r.Post("/episodes/{id}/watch", pageHandler.HandleWatchEpisode)
			r.Post("/episodes/{id}/watched-at", pageHandler.HandleEpisodeWatchedAt)
			r.Post("/episodes/{id}/notes", pageHandler.HandleEpisodeNotes)
			r.Post("/episodes/{id}/watch-up-to", pageHandler.HandleWatchUpTo)
			r.Post("/episodes/{id}/unwatch-up-to", pageHandler.HandleUnwatchUpTo)
			r.Post("/episodes/{id}/unwatch", pageHandler.HandleUnwatchEpisode)
			r.Post("/episodes/{id}/rate", pageHandler.HandleRateEpisode)
			r.Post("/seasons/{id}/watch-all", pageHandler.HandleWatchAllSeason)
			r.Post("/seasons/{id}/rate", pageHandler.HandleRateSeason)

			r.Get("/search", pageHandler.Search)
			r.Get("/preview", pageHandler.Preview)
			r.Post("/search/add", pageHandler.HandleSearchAdd)
			r.Post("/search/add-and-view", pageHandler.HandleSearchAddAndView)

			r.Get("/calendar", pageHandler.Calendar)

			r.Get("/discover", pageHandler.Discover)
			r.Post("/discover/add", pageHandler.HandleDiscoverAdd)
			r.Post("/discover/dismiss", pageHandler.HandleDismissRecommendation)

			r.Get("/suggestions", pageHandler.Suggestions)
			r.Post("/suggestions/{id}/add", pageHandler.HandleSuggestionAdd)
			r.Post("/suggestions/{id}/dismiss", pageHandler.HandleSuggestionDismiss)
			r.Post("/suggestions/{kwId}/dismiss-all", pageHandler.HandleSuggestionDismissAll)
			r.Post("/suggestions/{id}/restore", pageHandler.HandleSuggestionRestore)

			r.Get("/keywords", pageHandler.Keywords)
			r.Post("/keywords/new", pageHandler.HandleKeywordCreate)
			r.Post("/keywords/{id}/update", pageHandler.HandleKeywordUpdate)
			r.Post("/keywords/{id}/delete", pageHandler.HandleKeywordDelete)

			r.Get("/admin", pageHandler.Admin)
			r.Post("/admin/role", pageHandler.HandleSetRole)
			r.Post("/admin/delete", pageHandler.HandleDeleteUser)
			r.Post("/admin/tmdb-jobs/retry", pageHandler.HandleTMDBJobRetry)
			r.Post("/admin/tmdb-jobs/delete", pageHandler.HandleTMDBJobDelete)

			r.Get("/tags", pageHandler.TagsPage)
			r.Post("/tags/new", pageHandler.HandleTagPageCreate)
			r.Post("/tags/{id}/delete", pageHandler.HandleTagPageDelete)

			r.Get("/settings", pageHandler.Settings)
			r.Post("/settings", pageHandler.HandleSettings)
			r.Post("/settings/password", pageHandler.HandleChangePassword)
			r.Post("/settings/tags/new", pageHandler.HandleTagCreate)
			r.Post("/settings/tags/{id}/delete", pageHandler.HandleTagDelete)

			r.Get("/webhooks", pageHandler.Webhooks)
			r.Get("/webhooks/{id}", pageHandler.WebhookDetail)
			r.Get("/webhooks/{id}/edit", pageHandler.WebhookEdit)
			r.Post("/webhooks/new", pageHandler.HandleWebhookCreate)
			r.Post("/webhooks/{id}/update", pageHandler.HandleWebhookUpdate)
			r.Post("/webhooks/{id}/delete", pageHandler.HandleWebhookDelete)
			r.Post("/webhooks/{id}/test", pageHandler.HandleWebhookTest)

			r.Post("/settings/ics/regenerate", pageHandler.HandleRegenerateICS)
			r.Post("/settings/trakt/import", pageHandler.HandleTraktImport)
			r.Get("/settings/export", pageHandler.HandleLibraryExport)
			r.Post("/settings/import", pageHandler.HandleLibraryImport)
		})
	}

	// Background jobs.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	metadataSync := jobs.NewMetadataSync(librarySvc, cfg.MetadataSyncDuration())
	go metadataSync.Run(ctx)

	keywordScanner := jobs.NewKeywordScanner(keywordWatchRepo, libraryRepo, tmdbClient, cfg.KeywordScanDuration())
	go keywordScanner.Run(ctx)

	releaseNotifier := jobs.NewReleaseNotifier(libraryRepo, webhookSvc, cfg.MetadataSyncDuration())
	go releaseNotifier.Run(ctx)

	// Keep the discovery service's in-memory caches hot so users never
	// hit a cold TMDB path. Re-warms every 5h; trending TTL is 6h.
	discoveryWarmer := jobs.NewDiscoveryWarmer(discoverySvc, userRepo, 5*time.Hour)
	go discoveryWarmer.Run(ctx)

	// Re-run any tmdb_jobs that landed in status=failed so a transient
	// TMDB outage at add time doesn't leave library rows stuck at
	// "refreshing…" forever.
	go tmdbJobRunner.RunRetryLoop(ctx, 15*time.Minute)

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n, err := sessionRepo.DeleteExpired(context.Background())
				if err != nil {
					slog.Error("session cleanup failed", "error", err)
				} else if n > 0 {
					slog.Info("cleaned up expired sessions", "count", n)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("listening", "addr", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("shutting down")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("stopped")
}
