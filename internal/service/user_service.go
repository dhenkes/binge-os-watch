package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// UserServiceImpl implements model.UserService.
type UserServiceImpl struct {
	users  model.UserRepository
	hasher model.PasswordHasher
}

var _ model.UserService = (*UserServiceImpl)(nil)

// NewUserService creates a new UserServiceImpl.
func NewUserService(users model.UserRepository, hasher model.PasswordHasher) *UserServiceImpl {
	return &UserServiceImpl{users: users, hasher: hasher}
}

// Register creates a new user.
func (s *UserServiceImpl) Register(ctx context.Context, user *model.User) error {
	if err := user.Validate(); err != nil {
		return err
	}

	hash, err := s.hasher.Hash(user.Password)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	user.Password = hash
	user.Role = model.UserRoleUser

	return s.users.Create(ctx, user)
}

// Login authenticates a user by username and password.
func (s *UserServiceImpl) Login(ctx context.Context, username, password string) (*model.User, error) {
	user, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		return nil, model.NewUnauthenticated("invalid credentials")
	}

	ok, err := s.hasher.Verify(password, user.Password)
	if err != nil || !ok {
		return nil, model.NewUnauthenticated("invalid credentials")
	}

	return user, nil
}

// GetByID returns a user by ID.
func (s *UserServiceImpl) GetByID(ctx context.Context, id string) (*model.User, error) {
	return s.users.GetByID(ctx, id)
}

// ChangePassword verifies the current password and updates to the new one.
func (s *UserServiceImpl) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	ok, err := s.hasher.Verify(currentPassword, user.Password)
	if err != nil || !ok {
		return model.NewInvalidArgument("current password is incorrect")
	}

	if len(newPassword) < 8 {
		v := model.NewValidationErrors()
		v.Add("new_password", "must be at least 8 characters")
		return v.OrNil()
	}

	hash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("hashing new password: %w", err)
	}

	return s.users.UpdatePassword(ctx, userID, hash)
}

// GetSettings returns a user's settings.
func (s *UserServiceImpl) GetSettings(ctx context.Context, userID string) (*model.UserSettings, error) {
	return s.users.GetSettings(ctx, userID)
}

// UpdateSettings updates a user's settings using the update mask.
func (s *UserServiceImpl) UpdateSettings(ctx context.Context, settings *model.UserSettings, updateMask []string) error {
	current, err := s.users.GetSettings(ctx, settings.UserID)
	if err != nil {
		return err
	}

	for _, field := range updateMask {
		switch field {
		case "locale":
			current.Locale = settings.Locale
		case "theme":
			current.Theme = settings.Theme
		case "region":
			current.Region = settings.Region
		}
	}

	if err := current.Validate(); err != nil {
		return err
	}

	return s.users.UpsertSettings(ctx, current)
}

// ListAll returns all users.
func (s *UserServiceImpl) ListAll(ctx context.Context) ([]model.User, error) {
	return s.users.ListAll(ctx)
}

// DeleteUser deletes a user. Actor must be admin and cannot delete self.
func (s *UserServiceImpl) DeleteUser(ctx context.Context, actorID, targetID string) error {
	if actorID == targetID {
		return model.NewInvalidArgument("cannot delete your own account")
	}
	actor, err := s.users.GetByID(ctx, actorID)
	if err != nil {
		return err
	}
	if actor.Role != model.UserRoleAdmin {
		return model.NewPermissionDenied("admin role required")
	}
	return s.users.Delete(ctx, targetID)
}

// SetRole changes a user's role. Actor must be admin and cannot change own role.
func (s *UserServiceImpl) SetRole(ctx context.Context, actorID, targetID string, role model.UserRole) error {
	if actorID == targetID {
		return model.NewInvalidArgument("cannot change your own role")
	}
	actor, err := s.users.GetByID(ctx, actorID)
	if err != nil {
		return err
	}
	if actor.Role != model.UserRoleAdmin {
		return model.NewPermissionDenied("admin role required")
	}
	return s.users.UpdateRole(ctx, targetID, role)
}

// RegenerateICSToken generates a new ICS calendar token for the user.
func (s *UserServiceImpl) RegenerateICSToken(ctx context.Context, userID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating ICS token: %w", err)
	}
	token := hex.EncodeToString(b)

	settings, err := s.users.GetSettings(ctx, userID)
	if err != nil {
		return "", err
	}
	settings.ICSToken = token
	if err := s.users.UpsertSettings(ctx, settings); err != nil {
		return "", err
	}
	return token, nil
}

// GetByICSToken returns the user associated with the given ICS token.
func (s *UserServiceImpl) GetByICSToken(ctx context.Context, token string) (*model.User, error) {
	return s.users.GetByICSToken(ctx, token)
}
