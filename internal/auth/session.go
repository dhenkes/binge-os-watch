package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dhenkes/argon2id"
	"github.com/dhenkes/binge-os-watch/internal/model"
)

var (
	// ErrUserIDRequired is returned by Create if no userID was provided.
	ErrUserIDRequired = errors.New("session: userID must not be empty")
)

const (
	cookieName   = "session"
	tokenByteLen = 32
)

// SessionManager implements model.SessionManager.
type SessionManager struct {
	repo     model.SessionRepository
	duration time.Duration
	secure   bool
}

var _ model.SessionManager = (*SessionManager)(nil)

func NewSessionManager(repo model.SessionRepository, duration time.Duration, secureCookie bool) *SessionManager {
	return &SessionManager{
		repo:     repo,
		duration: duration,
		secure:   secureCookie,
	}
}

func (sm *SessionManager) Create(ctx context.Context, w http.ResponseWriter, userID string) (*model.Session, error) {
	if userID == "" {
		return nil, ErrUserIDRequired
	}

	token, err := argon2id.RandomToken(tokenByteLen)
	if err != nil {
		return nil, fmt.Errorf("generating session token: %w", err)
	}

	now := time.Now().UTC()
	session := &model.Session{
		UserID:     userID,
		Token:      token,
		CreatedAt:  now,
		ExpiresAt:  now.Add(sm.duration),
		LastSeenAt: now,
	}

	if err := sm.repo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}

	sm.setCookie(w, token, sm.duration)
	return session, nil
}

func (sm *SessionManager) Validate(ctx context.Context, r *http.Request) (string, error) {
	token := sm.extractToken(r)
	if token == "" {
		return "", model.NewUnauthenticated("missing session cookie or bearer token")
	}

	session, err := sm.repo.GetByToken(ctx, token)
	if err != nil {
		return "", model.NewUnauthenticated("invalid session")
	}

	if time.Now().UTC().After(session.ExpiresAt) {
		_ = sm.repo.Delete(ctx, session.ID)
		return "", model.NewUnauthenticated("session expired")
	}

	// Only extend the session if the last activity was more than 5 minutes ago.
	now := time.Now().UTC()
	if now.Sub(session.LastSeenAt) > 5*time.Minute {
		newExpiry := now.Add(sm.duration)
		if err := sm.repo.Extend(ctx, session.ID, newExpiry, now); err != nil {
			slog.Warn("failed to extend session", "session_id", session.ID, "error", err)
		}
	}

	return session.UserID, nil
}

func (sm *SessionManager) Destroy(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}

	session, err := sm.repo.GetByToken(ctx, cookie.Value)
	if err != nil {
		sm.clearCookie(w)
		return nil
	}

	if err := sm.repo.Delete(ctx, session.ID); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}

	sm.clearCookie(w)
	return nil
}

func (sm *SessionManager) extractToken(r *http.Request) string {
	if cookie, err := r.Cookie(cookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func (sm *SessionManager) setCookie(w http.ResponseWriter, token string, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   sm.secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (sm *SessionManager) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   sm.secure,
		SameSite: http.SameSiteStrictMode,
	})
}
