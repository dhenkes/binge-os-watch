package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/google/uuid"
)

// UserRepository implements model.UserRepository using SQLite.
type UserRepository struct {
	repo
}

var _ model.UserRepository = (*UserRepository)(nil)

// NewUserRepository creates a new UserRepository.
func NewUserRepository(db DBTX) *UserRepository {
	return &UserRepository{repo{db: db}}
}

// Create inserts a new user.
func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	user.ID = uuid.NewString()
	user.CreatedAt = time.Now().UTC()

	_, err := r.conn(ctx).ExecContext(ctx,
		`INSERT INTO users (id, username, password, role, created_at) VALUES (?, ?, ?, ?, ?)`,
		user.ID, user.Username, user.Password, string(user.Role), toUnix(user.CreatedAt),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.NewAlreadyExists("username already taken")
		}
		return fmt.Errorf("creating user: %w", err)
	}
	return nil
}

// GetByID returns a user by ID.
func (r *UserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	var u model.User
	var role string
	var createdAt int64
	err := r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, username, password, role, created_at FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.Password, &role, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.NewNotFound("user not found")
		}
		return nil, fmt.Errorf("getting user: %w", err)
	}
	u.Role = model.UserRole(role)
	u.CreatedAt = fromUnix(createdAt)
	return &u, nil
}

// GetByUsername returns a user by username.
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	var role string
	var createdAt int64
	err := r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, username, password, role, created_at FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.Password, &role, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.NewNotFound("user not found")
		}
		return nil, fmt.Errorf("getting user by username: %w", err)
	}
	u.Role = model.UserRole(role)
	u.CreatedAt = fromUnix(createdAt)
	return &u, nil
}

// ListAll returns all users ordered by creation time.
func (r *UserRepository) ListAll(ctx context.Context) ([]model.User, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT id, username, password, role, created_at FROM users ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		var role string
		var createdAt int64
		if err := rows.Scan(&u.ID, &u.Username, &u.Password, &role, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning user: %w", err)
		}
		u.Role = model.UserRole(role)
		u.CreatedAt = fromUnix(createdAt)
		users = append(users, u)
	}
	return users, rows.Err()
}

// Count returns the total number of users.
func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.conn(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting users: %w", err)
	}
	return count, nil
}

// UpdatePassword updates a user's password hash.
func (r *UserRepository) UpdatePassword(ctx context.Context, userID, hash string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE users SET password = ? WHERE id = ?`, hash, userID,
	)
	if err != nil {
		return fmt.Errorf("updating password: %w", err)
	}
	return nil
}

// UpdateRole changes a user's role.
func (r *UserRepository) UpdateRole(ctx context.Context, userID string, role model.UserRole) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE users SET role = ? WHERE id = ?`, string(role), userID,
	)
	if err != nil {
		return fmt.Errorf("updating role: %w", err)
	}
	return nil
}

// Delete removes a user and all associated data (cascades via FK).
func (r *UserRepository) Delete(ctx context.Context, userID string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`DELETE FROM users WHERE id = ?`, userID,
	)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	return nil
}

// GetSettings returns a user's settings, with defaults if none exist.
func (r *UserRepository) GetSettings(ctx context.Context, userID string) (*model.UserSettings, error) {
	var s model.UserSettings
	var updatedAt int64
	err := r.conn(ctx).QueryRowContext(ctx,
		`SELECT user_id, locale, theme, region, ics_token, updated_at FROM user_settings WHERE user_id = ?`, userID,
	).Scan(&s.UserID, &s.Locale, &s.Theme, &s.Region, &s.ICSToken, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &model.UserSettings{UserID: userID, Locale: "en", Theme: "dark", Region: "NL"}, nil
		}
		return nil, fmt.Errorf("getting user settings: %w", err)
	}
	s.UpdatedAt = fromUnix(updatedAt)
	return &s, nil
}

// UpsertSettings creates or updates a user's settings.
func (r *UserRepository) UpsertSettings(ctx context.Context, settings *model.UserSettings) error {
	settings.UpdatedAt = time.Now().UTC()

	_, err := r.conn(ctx).ExecContext(ctx,
		`INSERT INTO user_settings (user_id, locale, theme, region, ics_token, updated_at) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET locale=excluded.locale, theme=excluded.theme, region=excluded.region, ics_token=excluded.ics_token, updated_at=excluded.updated_at`,
		settings.UserID, settings.Locale, settings.Theme, settings.Region, settings.ICSToken, toUnix(settings.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("upserting user settings: %w", err)
	}
	return nil
}

// GetByICSToken returns the user associated with the given ICS calendar token.
func (r *UserRepository) GetByICSToken(ctx context.Context, token string) (*model.User, error) {
	var u model.User
	var role string
	var createdAt int64
	err := r.conn(ctx).QueryRowContext(ctx,
		`SELECT u.id, u.username, u.password, u.role, u.created_at
		 FROM users u JOIN user_settings us ON us.user_id = u.id
		 WHERE us.ics_token = ? AND us.ics_token != ''`, token,
	).Scan(&u.ID, &u.Username, &u.Password, &role, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.NewNotFound("invalid ICS token")
		}
		return nil, fmt.Errorf("getting user by ICS token: %w", err)
	}
	u.Role = model.UserRole(role)
	u.CreatedAt = fromUnix(createdAt)
	return &u, nil
}
