// Settings, import/export, and ICS pages.
package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/dhenkes/binge-os-watch/internal/service"
)

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

// --- ICS feed ---

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
