package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/google/uuid"
)

// WebhookRepository implements model.WebhookRepository using SQLite.
type WebhookRepository struct {
	repo
}

var _ model.WebhookRepository = (*WebhookRepository)(nil)

func NewWebhookRepository(db DBTX) *WebhookRepository {
	return &WebhookRepository{repo{db: db}}
}

func (r *WebhookRepository) Create(ctx context.Context, wh *model.Webhook) error {
	wh.ID = uuid.NewString()
	wh.CreatedAt = time.Now().UTC()

	if wh.Service == "" {
		wh.Service = model.WebhookServiceGeneric
	}
	_, err := r.conn(ctx).ExecContext(ctx,
		`INSERT INTO webhooks (id, user_id, name, url, events, service, body_template, headers, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		wh.ID, wh.UserID, wh.Name, wh.URL, wh.Events, wh.Service, wh.BodyTemplate, wh.Headers, toUnix(wh.CreatedAt),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.NewAlreadyExists("webhook URL already registered")
		}
		return fmt.Errorf("creating webhook: %w", err)
	}
	return nil
}

func (r *WebhookRepository) Update(ctx context.Context, wh *model.Webhook) error {
	_, err := r.conn(ctx).ExecContext(ctx,
		`UPDATE webhooks SET name = ?, url = ?, events = ?, service = ?, body_template = ?, headers = ? WHERE id = ?`,
		wh.Name, wh.URL, wh.Events, wh.Service, wh.BodyTemplate, wh.Headers, wh.ID,
	)
	if err != nil {
		return fmt.Errorf("updating webhook: %w", err)
	}
	return nil
}

func (r *WebhookRepository) ListByUser(ctx context.Context, userID string) ([]model.Webhook, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT id, user_id, name, url, events, service, body_template, headers, created_at FROM webhooks WHERE user_id = ? ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []model.Webhook
	for rows.Next() {
		var wh model.Webhook
		var createdAt int64
		if err := rows.Scan(&wh.ID, &wh.UserID, &wh.Name, &wh.URL, &wh.Events, &wh.Service, &wh.BodyTemplate, &wh.Headers, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning webhook: %w", err)
		}
		wh.CreatedAt = fromUnix(createdAt)
		webhooks = append(webhooks, wh)
	}
	return webhooks, rows.Err()
}

func (r *WebhookRepository) Delete(ctx context.Context, id string) error {
	_, err := r.conn(ctx).ExecContext(ctx, `DELETE FROM webhooks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting webhook: %w", err)
	}
	return nil
}

func (r *WebhookRepository) ListByEvent(ctx context.Context, userID, event string) ([]model.Webhook, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT id, user_id, name, url, events, service, body_template, headers, created_at FROM webhooks WHERE user_id = ? AND events LIKE ?`,
		userID, "%"+event+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("listing webhooks by event: %w", err)
	}
	defer rows.Close()

	var webhooks []model.Webhook
	for rows.Next() {
		var wh model.Webhook
		var createdAt int64
		if err := rows.Scan(&wh.ID, &wh.UserID, &wh.Name, &wh.URL, &wh.Events, &wh.Service, &wh.BodyTemplate, &wh.Headers, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning webhook: %w", err)
		}
		wh.CreatedAt = fromUnix(createdAt)
		webhooks = append(webhooks, wh)
	}
	return webhooks, rows.Err()
}

func (r *WebhookRepository) CreateDelivery(ctx context.Context, d *model.WebhookDelivery) error {
	d.ID = uuid.NewString()
	d.CreatedAt = time.Now().UTC()
	_, err := r.conn(ctx).ExecContext(ctx,
		`INSERT INTO webhook_deliveries (id, webhook_id, event, status_code, error, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		d.ID, d.WebhookID, d.Event, d.StatusCode, d.Error, toUnix(d.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("creating webhook delivery: %w", err)
	}
	return nil
}

func (r *WebhookRepository) ListDeliveries(ctx context.Context, webhookID string, limit int) ([]model.WebhookDelivery, error) {
	rows, err := r.conn(ctx).QueryContext(ctx,
		`SELECT id, webhook_id, event, status_code, error, created_at FROM webhook_deliveries WHERE webhook_id = ? ORDER BY created_at DESC LIMIT ?`,
		webhookID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("listing deliveries: %w", err)
	}
	defer rows.Close()

	var deliveries []model.WebhookDelivery
	for rows.Next() {
		var d model.WebhookDelivery
		var createdAt int64
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.Event, &d.StatusCode, &d.Error, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning delivery: %w", err)
		}
		d.CreatedAt = fromUnix(createdAt)
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}
