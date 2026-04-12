package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

type mockSessionManager struct {
	ValidateFn func(ctx context.Context, r *http.Request) (string, error)
}

func (m *mockSessionManager) Create(context.Context, http.ResponseWriter, string) (*model.Session, error) {
	return nil, nil
}

func (m *mockSessionManager) Validate(ctx context.Context, r *http.Request) (string, error) {
	if m.ValidateFn != nil {
		return m.ValidateFn(ctx, r)
	}
	return "", nil
}

func (m *mockSessionManager) Destroy(context.Context, http.ResponseWriter, *http.Request) error {
	return nil
}

func TestAuth_ValidSession(t *testing.T) {
	sm := &mockSessionManager{
		ValidateFn: func(_ context.Context, _ *http.Request) (string, error) {
			return "u1", nil
		},
	}

	var gotUserID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := Auth(sm)(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if gotUserID != "u1" {
		t.Errorf("userID = %q, want u1", gotUserID)
	}
}

func TestAuth_InvalidSession(t *testing.T) {
	sm := &mockSessionManager{
		ValidateFn: func(_ context.Context, _ *http.Request) (string, error) {
			return "", errors.New("expired")
		},
	}

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := Auth(sm)(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if called {
		t.Error("inner handler should not be called")
	}

	var appErr model.AppError
	if err := json.NewDecoder(rec.Body).Decode(&appErr); err != nil {
		t.Fatal(err)
	}
	if appErr.Code != model.ErrorCodeUnauthenticated {
		t.Errorf("error code = %v, want Unauthenticated", appErr.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestUserIDFromContext_Empty(t *testing.T) {
	if id := UserIDFromContext(context.Background()); id != "" {
		t.Errorf("got %q, want empty", id)
	}
}

func TestUserIDFromContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), userIDKey, 42)
	if id := UserIDFromContext(ctx); id != "" {
		t.Errorf("got %q, want empty (wrong type)", id)
	}
}

func TestContextWithUserID(t *testing.T) {
	ctx := ContextWithUserID(context.Background(), "user-123")
	if got := UserIDFromContext(ctx); got != "user-123" {
		t.Errorf("got %q, want user-123", got)
	}
}

func TestLogger_PassesThrough(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("ok"))
	})

	handler := Logger(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want ok", rec.Body.String())
	}
}
