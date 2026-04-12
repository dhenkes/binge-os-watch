// Tag management pages.
package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *PageHandler) TagsPage(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	allTags, _ := h.tags.List(r.Context(), userID)

	sort := r.URL.Query().Get("sort")
	if sort == "newest" {
		// Reverse: newest first (List returns name-ordered).
		for i, j := 0, len(allTags)-1; i < j; i, j = i+1, j-1 {
			allTags[i], allTags[j] = allTags[j], allTags[i]
		}
	}

	counts, _ := h.tagRepo.CountsByUser(r.Context(), userID)
	if counts == nil {
		counts = map[string]int{}
	}

	data := h.baseData("tags", user, settings)
	data["AllTags"] = allTags
	data["TagCounts"] = counts
	data["Sort"] = sort
	h.render(w, "tags", r, data)
}

func (h *PageHandler) TagNewPage(w http.ResponseWriter, r *http.Request) {
	_, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	h.render(w, "tag_new", r, h.baseData("tags", user, settings))
}

func (h *PageHandler) TagEditPage(w http.ResponseWriter, r *http.Request) {
	_, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	tagID := chi.URLParam(r, "id")
	tag, err := h.tagRepo.GetByID(r.Context(), tagID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	data := h.baseData("tags", user, settings)
	data["Tag"] = tag
	h.render(w, "tag_edit", r, data)
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

func (h *PageHandler) HandleTagPageUpdate(w http.ResponseWriter, r *http.Request) {
	_, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	tagID := chi.URLParam(r, "id")
	name := r.FormValue("name")
	_ = h.tags.Update(r.Context(), tagID, name)
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
