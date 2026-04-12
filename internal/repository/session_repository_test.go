package repository

import (
	"context"
	"testing"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

func seedUser(t *testing.T, db interface{ ExecContext(context.Context, string, ...any) (interface{ RowsAffected() (int64, error) }, error) }, username string) string {
	// Use raw SQL to avoid circular dependency — repo tests need users.
	return ""
}

// seedUserRaw inserts a user directly and returns the ID.
func seedUserRaw(t *testing.T, repo *UserRepository, username string) string {
	t.Helper()
	u := &model.User{Username: username, Password: "hashed", Role: model.UserRoleUser}
	if err := repo.Create(context.Background(), u); err != nil {
		t.Fatalf("seeding user %q: %v", username, err)
	}
	return u.ID
}

func TestSessionRepository_CreateAndGetByToken(t *testing.T) {
	db := testDB(t)
	userRepo := NewUserRepository(db)
	sessionRepo := NewSessionRepository(db)
	ctx := context.Background()

	userID := seedUserRaw(t, userRepo, "sess_user")

	now := time.Now().UTC()
	s := &model.Session{
		UserID:     userID,
		Token:      "test-token-123",
		CreatedAt:  now,
		ExpiresAt:  now.Add(24 * time.Hour),
		LastSeenAt: now,
	}
	if err := sessionRepo.Create(ctx, s); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if s.ID == "" {
		t.Fatal("ID should be set")
	}

	got, err := sessionRepo.GetByToken(ctx, "test-token-123")
	if err != nil {
		t.Fatalf("GetByToken() error: %v", err)
	}
	if got.UserID != userID {
		t.Errorf("UserID = %q, want %q", got.UserID, userID)
	}
	if got.Token != "test-token-123" {
		t.Errorf("Token = %q, want test-token-123", got.Token)
	}
}

func TestSessionRepository_GetByToken_NotFound(t *testing.T) {
	db := testDB(t)
	sessionRepo := NewSessionRepository(db)

	_, err := sessionRepo.GetByToken(context.Background(), "bogus")
	if err == nil {
		t.Fatal("expected error for nonexistent token")
	}
}

func TestSessionRepository_Extend(t *testing.T) {
	db := testDB(t)
	userRepo := NewUserRepository(db)
	sessionRepo := NewSessionRepository(db)
	ctx := context.Background()

	userID := seedUserRaw(t, userRepo, "ext_user")
	now := time.Now().UTC()
	s := &model.Session{
		UserID:     userID,
		Token:      "extend-token",
		CreatedAt:  now,
		ExpiresAt:  now.Add(1 * time.Hour),
		LastSeenAt: now,
	}
	sessionRepo.Create(ctx, s)

	newExpiry := now.Add(48 * time.Hour)
	newSeen := now.Add(10 * time.Minute)
	if err := sessionRepo.Extend(ctx, s.ID, newExpiry, newSeen); err != nil {
		t.Fatalf("Extend() error: %v", err)
	}

	got, _ := sessionRepo.GetByToken(ctx, "extend-token")
	if got.ExpiresAt.Unix() != newExpiry.Unix() {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, newExpiry)
	}
}

func TestSessionRepository_Delete(t *testing.T) {
	db := testDB(t)
	userRepo := NewUserRepository(db)
	sessionRepo := NewSessionRepository(db)
	ctx := context.Background()

	userID := seedUserRaw(t, userRepo, "del_user")
	now := time.Now().UTC()
	s := &model.Session{
		UserID:     userID,
		Token:      "del-token",
		CreatedAt:  now,
		ExpiresAt:  now.Add(1 * time.Hour),
		LastSeenAt: now,
	}
	sessionRepo.Create(ctx, s)

	if err := sessionRepo.Delete(ctx, s.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	_, err := sessionRepo.GetByToken(ctx, "del-token")
	if err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestSessionRepository_DeleteExpired(t *testing.T) {
	db := testDB(t)
	userRepo := NewUserRepository(db)
	sessionRepo := NewSessionRepository(db)
	ctx := context.Background()

	userID := seedUserRaw(t, userRepo, "exp_user")
	past := time.Now().UTC().Add(-1 * time.Hour)
	future := time.Now().UTC().Add(24 * time.Hour)

	// Expired session.
	sessionRepo.Create(ctx, &model.Session{
		UserID: userID, Token: "expired", CreatedAt: past,
		ExpiresAt: past, LastSeenAt: past,
	})
	// Valid session.
	sessionRepo.Create(ctx, &model.Session{
		UserID: userID, Token: "valid", CreatedAt: past,
		ExpiresAt: future, LastSeenAt: past,
	})

	n, err := sessionRepo.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("DeleteExpired() error: %v", err)
	}
	if n != 1 {
		t.Errorf("deleted %d, want 1", n)
	}

	// Valid session should still exist.
	if _, err := sessionRepo.GetByToken(ctx, "valid"); err != nil {
		t.Errorf("valid session should still exist: %v", err)
	}
	// Expired session should be gone.
	if _, err := sessionRepo.GetByToken(ctx, "expired"); err == nil {
		t.Error("expired session should be deleted")
	}
}
