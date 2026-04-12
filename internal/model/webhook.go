package model

import (
	"context"
	"time"
)

// WebhookService types — built-in presets vs custom.
const (
	WebhookServiceGeneric = "generic"
	WebhookServiceDiscord = "discord"
	WebhookServiceSlack   = "slack"
	WebhookServiceNtfy    = "ntfy"
	WebhookServiceCustom  = "custom"
)

// Webhook event types.
const (
	WebhookEventStatusChanged  = "status_changed"
	WebhookEventWatched        = "watched"
	WebhookEventEpisodeWatched = "episode_watched"
	WebhookEventAdded          = "added"
	WebhookEventReleased       = "released"
)

// AllWebhookEvents lists all available event types.
var AllWebhookEvents = []string{
	WebhookEventAdded,
	WebhookEventStatusChanged,
	WebhookEventWatched,
	WebhookEventEpisodeWatched,
	WebhookEventReleased,
}

// Webhook represents a user-configured webhook endpoint.
type Webhook struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Name         string    `json:"name"`
	URL          string    `json:"url"`
	Events       string    `json:"events"`
	Service      string    `json:"service"`       // "generic" | "discord" | "slack" | "ntfy" | "custom"
	BodyTemplate string    `json:"body_template"` // only for "custom"
	Headers      string    `json:"headers"`       // JSON map, only for "custom"
	CreatedAt    time.Time `json:"created_at"`
}

// WebhookDelivery records a single webhook dispatch attempt.
type WebhookDelivery struct {
	ID         string    `json:"id"`
	WebhookID  string    `json:"webhook_id"`
	Event      string    `json:"event"`
	StatusCode int       `json:"status_code"`
	Error      string    `json:"error"`
	CreatedAt  time.Time `json:"created_at"`
}

// WebhookService defines webhook dispatch operations.
type WebhookService interface {
	Dispatch(ctx context.Context, userID, event string, payload any)
	DispatchTest(ctx context.Context, wh Webhook)
}

// WebhookRepository defines persistence operations for webhooks.
type WebhookRepository interface {
	Create(ctx context.Context, wh *Webhook) error
	Update(ctx context.Context, wh *Webhook) error
	ListByUser(ctx context.Context, userID string) ([]Webhook, error)
	Delete(ctx context.Context, id string) error
	ListByEvent(ctx context.Context, userID, event string) ([]Webhook, error)
	CreateDelivery(ctx context.Context, d *WebhookDelivery) error
	ListDeliveries(ctx context.Context, webhookID string, limit int) ([]WebhookDelivery, error)
}
