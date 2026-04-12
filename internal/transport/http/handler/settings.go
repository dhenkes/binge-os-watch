package handler

import (
	"net/http"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

type SettingsHandler struct {
	users model.UserService
}

func NewSettingsHandler(users model.UserService) *SettingsHandler {
	return &SettingsHandler{users: users}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	settings, err := h.users.GetSettings(r.Context(), authUserID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var s model.UserSettings
	if err := decodeJSON(r, &s); err != nil {
		writeError(w, err)
		return
	}
	s.UserID = authUserID(r)
	if err := h.users.UpdateSettings(r.Context(), &s, updateMask(r)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}
