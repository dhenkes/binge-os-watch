package handler

// Auth pages.

import (
	"net/http"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

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
