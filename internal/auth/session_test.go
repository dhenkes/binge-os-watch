package auth

import (
  "context"
  "errors"
  "fmt"
  "net/http"
  "net/http/httptest"
  "testing"
  "time"

  "github.com/dhenkes/binge-os-watch/internal/model"
)

// stubSessionRepo is a minimal in-memory session store for tests.
type stubSessionRepo struct {
  sessions map[string]*model.Session // keyed by token
}

func newStubRepo() *stubSessionRepo {
  return &stubSessionRepo{sessions: map[string]*model.Session{}}
}

func (r *stubSessionRepo) Create(_ context.Context, s *model.Session) error {
  r.sessions[s.Token] = s
  return nil
}

func (r *stubSessionRepo) GetByToken(_ context.Context, token string) (*model.Session, error) {
  s, ok := r.sessions[token]
  if !ok {
    return nil, fmt.Errorf("not found")
  }
  return s, nil
}

func (r *stubSessionRepo) Extend(_ context.Context, id string, expiresAt, lastSeenAt time.Time) error {
  for _, s := range r.sessions {
    if s.ID == id {
      s.ExpiresAt = expiresAt
      s.LastSeenAt = lastSeenAt
      return nil
    }
  }
  return fmt.Errorf("not found")
}

func (r *stubSessionRepo) Delete(_ context.Context, id string) error {
  for token, s := range r.sessions {
    if s.ID == id {
      delete(r.sessions, token)
      return nil
    }
  }
  return nil
}

func (r *stubSessionRepo) DeleteExpired(_ context.Context) (int64, error) {
  return 0, nil
}

func TestCreate_SetsCookieAndStoresSession(t *testing.T) {
  repo := newStubRepo()
  sm := NewSessionManager(repo, 24*time.Hour, false)
  w := httptest.NewRecorder()

  session, err := sm.Create(context.Background(), w, "user-123")
  if err != nil {
    t.Fatalf("Create() error: %v", err)
  }
  if session.UserID != "user-123" {
    t.Errorf("UserID = %s, want user-123", session.UserID)
  }
  if session.Token == "" {
    t.Error("Token should not be empty")
  }

  cookies := w.Result().Cookies()
  if len(cookies) == 0 {
    t.Fatal("expected a cookie to be set")
  }
  cookie := cookies[0]
  if cookie.Name != "session" {
    t.Errorf("cookie name = %s, want session", cookie.Name)
  }
  if !cookie.HttpOnly {
    t.Error("cookie should be HttpOnly")
  }
  if cookie.SameSite != http.SameSiteStrictMode {
    t.Error("cookie should be SameSiteStrict")
  }
}

func TestCreate_EmptyUserID(t *testing.T) {
  repo := newStubRepo()
  sm := NewSessionManager(repo, 24*time.Hour, false)
  w := httptest.NewRecorder()

  _, err := sm.Create(context.Background(), w, "")
  if !errors.Is(err, ErrUserIDRequired) {
    t.Errorf("Create(\"\") error = %v, want ErrUserIDRequired", err)
  }
}

func TestCreate_SecureCookieFlag(t *testing.T) {
  repo := newStubRepo()
  sm := NewSessionManager(repo, 24*time.Hour, true)
  w := httptest.NewRecorder()

  _, _ = sm.Create(context.Background(), w, "user-123")
  cookie := w.Result().Cookies()[0]
  if !cookie.Secure {
    t.Error("cookie should be Secure when secureCookie=true")
  }
}

func TestValidate_ValidSession(t *testing.T) {
  repo := newStubRepo()
  sm := NewSessionManager(repo, 24*time.Hour, false)
  w := httptest.NewRecorder()

  session, _ := sm.Create(context.Background(), w, "user-456")

  req := httptest.NewRequest(http.MethodGet, "/", nil)
  req.AddCookie(&http.Cookie{Name: "session", Value: session.Token})

  userID, err := sm.Validate(context.Background(), req)
  if err != nil {
    t.Fatalf("Validate() error: %v", err)
  }
  if userID != "user-456" {
    t.Errorf("userID = %s, want user-456", userID)
  }
}

func TestValidate_MissingCookie(t *testing.T) {
  repo := newStubRepo()
  sm := NewSessionManager(repo, 24*time.Hour, false)
  req := httptest.NewRequest(http.MethodGet, "/", nil)

  _, err := sm.Validate(context.Background(), req)
  if err == nil {
    t.Error("Validate() should error when no cookie is present")
  }
}

func TestValidate_InvalidToken(t *testing.T) {
  repo := newStubRepo()
  sm := NewSessionManager(repo, 24*time.Hour, false)
  req := httptest.NewRequest(http.MethodGet, "/", nil)
  req.AddCookie(&http.Cookie{Name: "session", Value: "bogus-token"})

  _, err := sm.Validate(context.Background(), req)
  if err == nil {
    t.Error("Validate() should error for invalid token")
  }
}

func TestValidate_ExpiredSession(t *testing.T) {
  repo := newStubRepo()
  sm := NewSessionManager(repo, 1*time.Millisecond, false)
  w := httptest.NewRecorder()

  session, _ := sm.Create(context.Background(), w, "user-789")

  // Force expiry into the past.
  repo.sessions[session.Token].ExpiresAt = time.Now().UTC().Add(-1 * time.Hour)

  req := httptest.NewRequest(http.MethodGet, "/", nil)
  req.AddCookie(&http.Cookie{Name: "session", Value: session.Token})

  _, err := sm.Validate(context.Background(), req)
  if err == nil {
    t.Error("Validate() should error for expired session")
  }
}

func TestValidate_ExtendsSession_WhenLastSeenOver5MinAgo(t *testing.T) {
  repo := newStubRepo()
  sm := NewSessionManager(repo, 24*time.Hour, false)
  w := httptest.NewRecorder()

  session, _ := sm.Create(context.Background(), w, "user-ext")

  // Set LastSeenAt to 10 minutes ago to trigger extension.
  originalExpiry := repo.sessions[session.Token].ExpiresAt
  repo.sessions[session.Token].LastSeenAt = time.Now().UTC().Add(-10 * time.Minute)

  req := httptest.NewRequest(http.MethodGet, "/", nil)
  req.AddCookie(&http.Cookie{Name: "session", Value: session.Token})

  userID, err := sm.Validate(context.Background(), req)
  if err != nil {
    t.Fatalf("Validate() error: %v", err)
  }
  if userID != "user-ext" {
    t.Errorf("userID = %s, want user-ext", userID)
  }

  // Expiry should have been pushed forward.
  newExpiry := repo.sessions[session.Token].ExpiresAt
  if !newExpiry.After(originalExpiry) {
    t.Error("session expiry should be extended")
  }
}

func TestValidate_DoesNotExtend_WhenLastSeenRecent(t *testing.T) {
  repo := newStubRepo()
  sm := NewSessionManager(repo, 24*time.Hour, false)
  w := httptest.NewRecorder()

  session, _ := sm.Create(context.Background(), w, "user-noext")

  // LastSeenAt is now (just created), so no extension should happen.
  originalExpiry := repo.sessions[session.Token].ExpiresAt

  req := httptest.NewRequest(http.MethodGet, "/", nil)
  req.AddCookie(&http.Cookie{Name: "session", Value: session.Token})

  _, err := sm.Validate(context.Background(), req)
  if err != nil {
    t.Fatalf("Validate() error: %v", err)
  }

  newExpiry := repo.sessions[session.Token].ExpiresAt
  if !newExpiry.Equal(originalExpiry) {
    t.Error("session expiry should not change when last seen is recent")
  }
}

func TestDestroy_InvalidToken_ClearsCookie(t *testing.T) {
  repo := newStubRepo()
  sm := NewSessionManager(repo, 24*time.Hour, false)

  req := httptest.NewRequest(http.MethodGet, "/", nil)
  req.AddCookie(&http.Cookie{Name: "session", Value: "bogus-token"})

  w := httptest.NewRecorder()
  err := sm.Destroy(context.Background(), w, req)
  if err != nil {
    t.Fatalf("Destroy() error: %v", err)
  }

  cookies := w.Result().Cookies()
  if len(cookies) == 0 {
    t.Fatal("expected cookie to be cleared")
  }
  if cookies[0].MaxAge != -1 {
    t.Errorf("cookie MaxAge = %d, want -1", cookies[0].MaxAge)
  }
}

func TestDestroy_ClearsCookieAndDeletesSession(t *testing.T) {
  repo := newStubRepo()
  sm := NewSessionManager(repo, 24*time.Hour, false)
  w := httptest.NewRecorder()

  session, _ := sm.Create(context.Background(), w, "user-del")

  req := httptest.NewRequest(http.MethodGet, "/", nil)
  req.AddCookie(&http.Cookie{Name: "session", Value: session.Token})

  w2 := httptest.NewRecorder()
  err := sm.Destroy(context.Background(), w2, req)
  if err != nil {
    t.Fatalf("Destroy() error: %v", err)
  }

  if len(repo.sessions) != 0 {
    t.Error("session should be deleted from repo")
  }

  cookies := w2.Result().Cookies()
  if len(cookies) == 0 {
    t.Fatal("expected cookie to be cleared")
  }
  if cookies[0].MaxAge != -1 {
    t.Errorf("cookie MaxAge = %d, want -1", cookies[0].MaxAge)
  }
}

func TestDestroy_NoCookie_NoError(t *testing.T) {
  repo := newStubRepo()
  sm := NewSessionManager(repo, 24*time.Hour, false)
  req := httptest.NewRequest(http.MethodGet, "/", nil)
  w := httptest.NewRecorder()

  err := sm.Destroy(context.Background(), w, req)
  if err != nil {
    t.Errorf("Destroy() should not error when no cookie: %v", err)
  }
}
