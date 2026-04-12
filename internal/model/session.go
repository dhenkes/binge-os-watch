package model

import (
	"context"
	"net/http"
	"time"
)

// Session represents an authenticated user session.
type Session struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Token      string    `json:"-"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

// SessionRepository defines persistence operations for sessions.
type SessionRepository interface {
	Create(ctx context.Context, session *Session) error
	GetByToken(ctx context.Context, token string) (*Session, error)
	Extend(ctx context.Context, id string, expiresAt, lastSeenAt time.Time) error
	Delete(ctx context.Context, id string) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// SessionManager defines session lifecycle operations used by middleware.
type SessionManager interface {
	Create(ctx context.Context, w http.ResponseWriter, userID string) (*Session, error)
	Validate(ctx context.Context, r *http.Request) (string, error)
	Destroy(ctx context.Context, w http.ResponseWriter, r *http.Request) error
}

// TxFunc wraps a function in a database transaction.
type TxFunc func(ctx context.Context, fn func(ctx context.Context) error) error
