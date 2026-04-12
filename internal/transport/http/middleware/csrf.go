package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const (
	csrfCookieName = "csrf_token"
	csrfFieldName  = "csrf_token"
	csrfTokenLen   = 32
)

// CSRF implements double-submit cookie CSRF protection.
// GET/HEAD/OPTIONS requests get a CSRF cookie set (if not present).
// POST/PUT/PATCH/DELETE requests must include a form field matching the cookie.
// API requests (Authorization: Bearer) are exempt — CSRF is a browser-only attack.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API clients using Bearer tokens are not vulnerable to CSRF.
		if r.Header.Get("Authorization") != "" {
			next.ServeHTTP(w, r)
			return
		}

		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			// Ensure a CSRF cookie exists for subsequent form submissions.
			if _, err := r.Cookie(csrfCookieName); err != nil {
				setCSRFCookie(w)
			}
			next.ServeHTTP(w, r)

		default:
			// Validate: form field must match cookie.
			cookie, err := r.Cookie(csrfCookieName)
			if err != nil || cookie.Value == "" {
				http.Error(w, "missing CSRF token", http.StatusForbidden)
				return
			}

			// Check multipart forms (file uploads) and regular forms.
			formToken := r.FormValue(csrfFieldName)
			if formToken == "" || formToken != cookie.Value {
				http.Error(w, "invalid CSRF token", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		}
	})
}

func setCSRFCookie(w http.ResponseWriter) {
	token := generateCSRFToken()
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false, // Must be readable by forms (but not JS since script-src 'none').
		SameSite: http.SameSiteStrictMode,
	})
}

func generateCSRFToken() string {
	b := make([]byte, csrfTokenLen)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CSRFToken extracts the current CSRF token from the request cookie.
// Used by page handlers to inject into template data.
func CSRFToken(r *http.Request) string {
	if c, err := r.Cookie(csrfCookieName); err == nil {
		return c.Value
	}
	return ""
}
