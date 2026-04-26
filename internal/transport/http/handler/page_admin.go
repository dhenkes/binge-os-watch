// Admin pages.
package handler

import (
	"log/slog"
	"net/http"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

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

// HandleRecalcStatuses recalculates the derived status for every TV library
// entry across all users. Admin-only action on the admin page.
func (h *PageHandler) HandleRecalcStatuses(w http.ResponseWriter, r *http.Request) {
	if _, _, _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	users, err := h.users.ListAll(r.Context())
	if err != nil {
		slog.Error("recalc: listing users", "error", err)
		http.Redirect(w, r, "/admin?tab=stats", http.StatusSeeOther)
		return
	}
	var recalced int
	for _, u := range users {
		page := model.PageRequest{PageSize: 1000}
		result, err := h.libraryRepo.List(r.Context(), u.ID, model.LibraryFilter{MediaType: model.MediaTypeTV}, page)
		if err != nil {
			slog.Error("recalc: listing library", "user", u.ID, "error", err)
			continue
		}
		for _, v := range result.Items {
			if err := h.watch.RecalcStatus(r.Context(), v.Entry.ID); err != nil {
				slog.Error("recalc status", "entry", v.Entry.ID, "error", err)
				continue
			}
			recalced++
		}
	}
	slog.Info("recalc statuses done", "count", recalced)
	http.Redirect(w, r, "/admin?tab=stats", http.StatusSeeOther)
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
