package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// WebhookDispatcher fires webhook notifications.
type WebhookDispatcher interface {
	Dispatch(ctx context.Context, userID, event string, payload any)
}

// ReleaseNotifier dispatches `released` webhook events for library entries
// whose catalog release date has passed and that haven't been notified yet.
type ReleaseNotifier struct {
	library  model.LibraryRepository
	webhooks WebhookDispatcher
	interval time.Duration
}

func NewReleaseNotifier(library model.LibraryRepository, webhooks WebhookDispatcher, interval time.Duration) *ReleaseNotifier {
	return &ReleaseNotifier{library: library, webhooks: webhooks, interval: interval}
}

func (j *ReleaseNotifier) Run(ctx context.Context) {
	slog.Info("release notifier started", "interval", j.interval)

	j.notify(ctx)

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			j.notify(ctx)
		case <-ctx.Done():
			slog.Info("release notifier stopped")
			return
		}
	}
}

func (j *ReleaseNotifier) notify(ctx context.Context) {
	items, err := j.library.ListPendingReleaseNotifications(ctx)
	if err != nil {
		slog.Error("release notifier: listing", "error", err)
		return
	}
	if len(items) == 0 {
		return
	}

	slog.Info("release notifier dispatching", "count", len(items))
	for _, v := range items {
		payload := map[string]any{
			"media_id":   v.Entry.ID,
			"title":      v.Title(),
			"media_type": string(v.Entry.MediaType),
			"status":     string(v.Status),
		}
		j.webhooks.Dispatch(ctx, v.Entry.UserID, model.WebhookEventReleased, payload)
		if err := j.library.MarkReleaseNotified(ctx, v.Entry.ID); err != nil {
			slog.Warn("release notifier: marking notified", "media_id", v.Entry.ID, "error", err)
		}
	}
}
