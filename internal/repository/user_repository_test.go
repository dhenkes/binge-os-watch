package repository

import (
	"context"
	"testing"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

func TestUserRepository_CreateAndGetByID(t *testing.T) {
	db := testDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	u := &model.User{Username: "alice", Password: "hashed", Role: model.UserRoleUser}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if u.ID == "" {
		t.Fatal("ID should be set after Create")
	}
	if u.CreatedAt.IsZero() {
		t.Fatal("CreatedAt should be set after Create")
	}

	got, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got.Username != "alice" {
		t.Errorf("Username = %q, want alice", got.Username)
	}
	if got.Role != model.UserRoleUser {
		t.Errorf("Role = %q, want user", got.Role)
	}
}

func TestUserRepository_GetByUsername(t *testing.T) {
	db := testDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	u := &model.User{Username: "bob", Password: "hashed", Role: model.UserRoleUser}
	repo.Create(ctx, u)

	got, err := repo.GetByUsername(ctx, "bob")
	if err != nil {
		t.Fatalf("GetByUsername() error: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("ID = %q, want %q", got.ID, u.ID)
	}
}

func TestUserRepository_GetByUsername_NotFound(t *testing.T) {
	db := testDB(t)
	repo := NewUserRepository(db)

	_, err := repo.GetByUsername(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	db := testDB(t)
	repo := NewUserRepository(db)

	_, err := repo.GetByID(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

func TestUserRepository_DuplicateUsername(t *testing.T) {
	db := testDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	repo.Create(ctx, &model.User{Username: "dup", Password: "h", Role: model.UserRoleUser})
	err := repo.Create(ctx, &model.User{Username: "dup", Password: "h", Role: model.UserRoleUser})
	if err == nil {
		t.Fatal("expected already exists error")
	}
}

func TestUserRepository_UpdatePassword(t *testing.T) {
	db := testDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	u := &model.User{Username: "pw", Password: "old", Role: model.UserRoleUser}
	repo.Create(ctx, u)

	if err := repo.UpdatePassword(ctx, u.ID, "new"); err != nil {
		t.Fatalf("UpdatePassword() error: %v", err)
	}

	got, _ := repo.GetByID(ctx, u.ID)
	if got.Password != "new" {
		t.Errorf("Password = %q, want new", got.Password)
	}
}

func TestUserRepository_UpdateRole(t *testing.T) {
	db := testDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	u := &model.User{Username: "role", Password: "h", Role: model.UserRoleUser}
	repo.Create(ctx, u)

	if err := repo.UpdateRole(ctx, u.ID, model.UserRoleAdmin); err != nil {
		t.Fatalf("UpdateRole() error: %v", err)
	}

	got, _ := repo.GetByID(ctx, u.ID)
	if got.Role != model.UserRoleAdmin {
		t.Errorf("Role = %q, want admin", got.Role)
	}
}

func TestUserRepository_Delete(t *testing.T) {
	db := testDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	u := &model.User{Username: "del", Password: "h", Role: model.UserRoleUser}
	repo.Create(ctx, u)

	if err := repo.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	_, err := repo.GetByID(ctx, u.ID)
	if err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestUserRepository_Settings_DefaultOnMissing(t *testing.T) {
	db := testDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	u := &model.User{Username: "settings", Password: "h", Role: model.UserRoleUser}
	repo.Create(ctx, u)

	s, err := repo.GetSettings(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetSettings() error: %v", err)
	}
	if s.Locale != "en" {
		t.Errorf("Locale = %q, want en (default)", s.Locale)
	}
	if s.Theme != "dark" {
		t.Errorf("Theme = %q, want dark (default)", s.Theme)
	}
	if s.Region != "NL" {
		t.Errorf("Region = %q, want NL (default)", s.Region)
	}
}

func TestUserRepository_UpsertSettings(t *testing.T) {
	db := testDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	u := &model.User{Username: "upsert", Password: "h", Role: model.UserRoleUser}
	repo.Create(ctx, u)

	// First upsert (insert).
	s := &model.UserSettings{UserID: u.ID, Locale: "de", Theme: "oled", Region: "DE"}
	if err := repo.UpsertSettings(ctx, s); err != nil {
		t.Fatalf("UpsertSettings() error: %v", err)
	}

	got, _ := repo.GetSettings(ctx, u.ID)
	if got.Locale != "de" || got.Theme != "oled" || got.Region != "DE" {
		t.Errorf("settings = %+v, want de/oled/DE", got)
	}

	// Second upsert (update).
	s.Theme = "light"
	if err := repo.UpsertSettings(ctx, s); err != nil {
		t.Fatalf("UpsertSettings() update error: %v", err)
	}

	got, _ = repo.GetSettings(ctx, u.ID)
	if got.Theme != "light" {
		t.Errorf("Theme = %q, want light after update", got.Theme)
	}
}
