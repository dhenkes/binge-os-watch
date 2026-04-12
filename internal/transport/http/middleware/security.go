package middleware

import "net/http"

// Security sets HTTP security headers on every response.
// Defense-in-depth: even if a sanitization bug lets something through,
// CSP prevents it from executing.
func Security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		// Content Security Policy:
		// - default-src 'self': only load resources from this origin.
		// - script-src 'none': no JavaScript at all.
		// - style-src 'self' 'unsafe-inline': allow inline styles to prevent FOUC.
		// - img-src 'self' data:: allow local images and data: URIs.
		// - frame-src 'none': no iframes.
		// - object-src 'none': no plugins.
		// - base-uri 'self': prevent <base> tag hijacking.
		// - form-action 'self': forms can only submit to this origin.
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'none'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; "+
				"font-src 'self'; "+
				"frame-src 'none'; "+
				"object-src 'none'; "+
				"base-uri 'self'; "+
				"form-action 'self'",
		)

		// Send Referer for same-origin requests (needed for form redirects)
		// but strip it for cross-origin.
		h.Set("Referrer-Policy", "same-origin")

		// Prevent MIME-sniffing — browsers must respect Content-Type.
		h.Set("X-Content-Type-Options", "nosniff")

		// Prevent this page from being embedded in an iframe on another site.
		h.Set("X-Frame-Options", "DENY")

		// Opt out of Google's FLoC/Topics tracking.
		h.Set("Permissions-Policy", "interest-cohort=()")

		next.ServeHTTP(w, r)
	})
}
