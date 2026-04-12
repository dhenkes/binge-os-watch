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

// SessionRepository implements model.SessionRepository using SQLite.
type SessionRepository struct {
	repo
}

var _ model.SessionRepository = (*SessionRepository)(nil)

// NewSessionRepository creates a new SessionRepository.
func NewSessionRepository(db DBTX) *SessionRepository {
	return &SessionRepository{repo{db: db}}
}

// Create inserts a new session.
func (r *SessionRepository) Create(ctx context.Context, session *model.Session) error {
	session.ID = uuid.NewString()

	_, err := r.conn(ctx).ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, token, created_at, expires_at, last_seen_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		session.ID, session.UserID, session.Token,
		toUnix(session.CreatedAt), toUnix(session.ExpiresAt), toUnix(session.LastSeenAt),
	)
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}
	return nil
}

// GetByToken returns a session by its token.
func (r *SessionRepository) GetByToken(ctx context.Context, token string) (*model.Session, error) {
	var s model.Session
	var createdAt, expiresAt, lastSeenAt int64
	err := r.conn(ctx).QueryRowContext(ctx,
		`SELECT id, user_id, token, created_at, expires_at, last_seen_at
		 FROM sessions WHERE token = ?`, token,
	).Scan(&s.ID, &s.UserID, &s.Token, &createdAt, &expiresAt, &lastSeenAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.NewNotFound("session not found")
		}
		return nil, fmt.Errorf("getting session: %w", err)
	}
	s.CreatedAt = fromUnix(createdAt)
	s.ExpiresAt = fromUnix(expiresAt)
	s.LastSeenAt = fromUnix(lastSeenAt)
	return &s, nil
}

// Extend updates a session's expiry and last-seen timestamp (sliding window).
func (r *SessionRepository) Extend(ctx context.Context, id string, expiresAt, lastSeenAt time.Time) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE sessions SET expires_at = ?, last_seen_at = ? WHERE id = ?`,
		toUnix(expiresAt), toUnix(lastSeenAt), id,
	)
	if err != nil {
		return fmt.Errorf("extending session: %w", err)
	}
	return nil
}

// Delete removes a session by ID.
func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`DELETE FROM sessions WHERE id = ?`, id,
	)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

// DeleteExpired removes all expired sessions (housekeeping).
func (r *SessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	res, err := r.conn(ctx).ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at <= ?`, toUnix(time.Now().UTC()),
	)
	if err != nil {
		return 0, fmt.Errorf("deleting expired sessions: %w", err)
	}
	return res.RowsAffected()
}
