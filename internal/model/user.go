package model

import (
	"context"
	"strings"
	"time"
)

// UserRole represents the role of a user.
type UserRole string

const (
	UserRoleUser  UserRole = "user"
	UserRoleAdmin UserRole = "admin"
)

// ValidUserRoles is the set of valid roles for validation.
var ValidUserRoles = []UserRole{UserRoleUser, UserRoleAdmin}

// User represents a registered user.
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"-"`
	Role      UserRole  `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// Validate checks required fields on User for registration.
func (u *User) Validate() error {
	v := NewValidationErrors()
	if strings.TrimSpace(u.Username) == "" {
		v.Add("username", "must not be empty")
	}
	if u.Password == "" {
		v.Add("password", "must not be empty")
	} else if len(u.Password) < 8 {
		v.Add("password", "must be at least 8 characters")
	}
	if u.Role != "" {
		valid := false
		for _, r := range ValidUserRoles {
			if u.Role == r {
				valid = true
				break
			}
		}
		if !valid {
			v.Add("role", "must be one of: user, admin")
		}
	}
	return v.OrNil()
}

// UserSettings holds per-user configuration.
type UserSettings struct {
	UserID    string    `json:"user_id"`
	Locale    string    `json:"locale"`
	Theme     string    `json:"theme"`
	Region    string    `json:"region"`
	ICSToken  string    `json:"ics_token"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks user settings.
func (s *UserSettings) Validate() error {
	v := NewValidationErrors()
	if s.Locale != "en" && s.Locale != "de" && s.Locale != "nl" {
		v.Add("locale", "must be 'en', 'de', or 'nl'")
	}
	if s.Theme != "light" && s.Theme != "dark" && s.Theme != "oled" {
		v.Add("theme", "must be 'light', 'dark', or 'oled'")
	}
	if s.Region != "" && len(s.Region) != 2 {
		v.Add("region", "must be a 2-letter ISO 3166-1 code")
	}
	return v.OrNil()
}

// UserRepository defines persistence operations for users.
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	ListAll(ctx context.Context) ([]User, error)
	Count(ctx context.Context) (int, error)
	UpdatePassword(ctx context.Context, userID, hash string) error
	UpdateRole(ctx context.Context, userID string, role UserRole) error
	Delete(ctx context.Context, userID string) error
	GetSettings(ctx context.Context, userID string) (*UserSettings, error)
	UpsertSettings(ctx context.Context, settings *UserSettings) error
	GetByICSToken(ctx context.Context, token string) (*User, error)
}

// PasswordHasher defines password hashing and verification.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, hash string) (bool, error)
}

// UserService defines business logic for users.
type UserService interface {
	Register(ctx context.Context, user *User) error
	Login(ctx context.Context, username, password string) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
	ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error
	GetSettings(ctx context.Context, userID string) (*UserSettings, error)
	UpdateSettings(ctx context.Context, settings *UserSettings, updateMask []string) error
	ListAll(ctx context.Context) ([]User, error)
	DeleteUser(ctx context.Context, actorID, targetID string) error
	SetRole(ctx context.Context, actorID, targetID string, role UserRole) error
	RegenerateICSToken(ctx context.Context, userID string) (string, error)
	GetByICSToken(ctx context.Context, token string) (*User, error)
}
