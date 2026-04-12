// Webhook pages.
package handler

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

func (h *PageHandler) Webhooks(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	webhooks, _ := h.webhookRepo.ListByUser(r.Context(), userID)
	data := h.baseData("webhooks", user, settings)
	data["Webhooks"] = webhooks
	data["Msg"] = r.URL.Query().Get("msg")
	h.render(w, "webhooks", r, data)
}

func (h *PageHandler) WebhookDetail(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	whID := chi.URLParam(r, "id")
	webhooks, _ := h.webhookRepo.ListByUser(r.Context(), userID)
	var target *model.Webhook
	for i := range webhooks {
		if webhooks[i].ID == whID {
			target = &webhooks[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	deliveries, _ := h.webhookRepo.ListDeliveries(r.Context(), whID, 50)
	data := h.baseData("webhooks", user, settings)
	data["Webhook"] = target
	data["Deliveries"] = deliveries
	h.render(w, "webhook_detail", r, data)
}

func (h *PageHandler) WebhookEdit(w http.ResponseWriter, r *http.Request) {
	userID, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	whID := chi.URLParam(r, "id")
	webhooks, _ := h.webhookRepo.ListByUser(r.Context(), userID)
	var target *model.Webhook
	for i := range webhooks {
		if webhooks[i].ID == whID {
			target = &webhooks[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	events := strings.Split(target.Events, ",")
	hasEvent := func(e string) bool {
		for _, ev := range events {
			if strings.TrimSpace(ev) == e {
				return true
			}
		}
		return false
	}
	data := h.baseData("webhooks", user, settings)
	data["Webhook"] = target
	data["Msg"] = r.URL.Query().Get("msg")
	data["HasAdded"] = hasEvent(model.WebhookEventAdded)
	data["HasStatusChanged"] = hasEvent(model.WebhookEventStatusChanged)
	data["HasWatched"] = hasEvent(model.WebhookEventWatched)
	data["HasEpisodeWatched"] = hasEvent(model.WebhookEventEpisodeWatched)
	data["HasReleased"] = hasEvent(model.WebhookEventReleased)
	h.render(w, "webhook_edit", r, data)
}

func (h *PageHandler) HandleWebhookUpdate(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	whID := chi.URLParam(r, "id")
	webhooks, _ := h.webhookRepo.ListByUser(r.Context(), userID)
	var found bool
	for _, wh := range webhooks {
		if wh.ID == whID {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	rawURL := r.FormValue("url")
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		http.Redirect(w, r, "/webhooks/"+whID+"/edit?msg=webhook_error", http.StatusSeeOther)
		return
	}
	svc := r.FormValue("service")
	if svc == "" {
		svc = model.WebhookServiceGeneric
	}
	r.ParseForm()
	events := strings.Join(r.Form["events"], ",")
	if events == "" {
		events = model.WebhookEventStatusChanged
	}
	wh := &model.Webhook{
		ID:           whID,
		Name:         r.FormValue("name"),
		URL:          rawURL,
		Events:       events,
		Service:      svc,
		BodyTemplate: r.FormValue("body_template"),
		Headers:      r.FormValue("headers"),
	}
	if err := h.webhookRepo.Update(r.Context(), wh); err != nil {
		http.Redirect(w, r, "/webhooks/"+whID+"/edit?msg=webhook_error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/webhooks/"+whID, http.StatusSeeOther)
}

func (h *PageHandler) WebhookNewType(w http.ResponseWriter, r *http.Request) {
	_, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	data := h.baseData("webhooks", user, settings)
	h.render(w, "webhook_new_type", r, data)
}

func (h *PageHandler) WebhookNew(w http.ResponseWriter, r *http.Request) {
	_, user, settings, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	svc := r.URL.Query().Get("service")
	labels := map[string]string{
		model.WebhookServiceGeneric: "Generic JSON",
		model.WebhookServiceDiscord: "Discord",
		model.WebhookServiceSlack:   "Slack",
		model.WebhookServiceNtfy:    "ntfy",
		model.WebhookServiceCustom:  "Custom",
	}
	label, ok2 := labels[svc]
	if !ok2 {
		http.Redirect(w, r, "/webhooks/new-type", http.StatusSeeOther)
		return
	}
	data := h.baseData("webhooks", user, settings)
	data["Service"] = svc
	data["ServiceLabel"] = label
	data["Msg"] = r.URL.Query().Get("msg")
	h.render(w, "webhook_new", r, data)
}

func (h *PageHandler) HandleWebhookCreate(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	rawURL := r.FormValue("url")
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		http.Redirect(w, r, "/webhooks?msg=webhook_error", http.StatusSeeOther)
		return
	}
	svc := r.FormValue("service")
	if svc == "" {
		svc = model.WebhookServiceGeneric
	}
	r.ParseForm()
	events := strings.Join(r.Form["events"], ",")
	if events == "" {
		events = model.WebhookEventStatusChanged
	}
	wh := &model.Webhook{
		UserID:       userID,
		Name:         r.FormValue("name"),
		URL:          rawURL,
		Events:       events,
		Service:      svc,
		BodyTemplate: r.FormValue("body_template"),
		Headers:      r.FormValue("headers"),
	}
	if err := h.webhookRepo.Create(r.Context(), wh); err != nil {
		http.Redirect(w, r, "/webhooks?msg=webhook_error", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/webhooks", http.StatusSeeOther)
}

func (h *PageHandler) HandleWebhookDelete(w http.ResponseWriter, r *http.Request) {
	_, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	whID := chi.URLParam(r, "id")
	_ = h.webhookRepo.Delete(r.Context(), whID)
	http.Redirect(w, r, "/webhooks", http.StatusSeeOther)
}

func (h *PageHandler) HandleWebhookTest(w http.ResponseWriter, r *http.Request) {
	userID, _, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	whID := chi.URLParam(r, "id")
	webhooks, _ := h.webhookRepo.ListByUser(r.Context(), userID)
	for _, wh := range webhooks {
		if wh.ID == whID {
			go h.webhookSvc.DispatchTest(context.Background(), wh)
			break
		}
	}
	http.Redirect(w, r, "/webhooks/"+whID, http.StatusSeeOther)
}
