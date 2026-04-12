package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"text/template"
	"time"

	"github.com/dhenkes/binge-os-watch/internal/model"
)

// WebhookServiceImpl implements webhook dispatch with per-service templates.
type WebhookServiceImpl struct {
	webhooks model.WebhookRepository
}

func NewWebhookService(webhooks model.WebhookRepository) *WebhookServiceImpl {
	return &WebhookServiceImpl{webhooks: webhooks}
}

// servicePresets holds the built-in body template + content type per service.
type servicePreset struct {
	bodyTemplate string
	contentType  string
	headers      map[string]string
}

var presets = map[string]servicePreset{
	model.WebhookServiceDiscord: {
		bodyTemplate: `{"content":"**{{.Title}}** is now {{.Status}} ({{.MediaType}})"}`,
		contentType:  "application/json",
	},
	model.WebhookServiceSlack: {
		bodyTemplate: `{"text":"*{{.Title}}* is now {{.Status}} ({{.MediaType}})"}`,
		contentType:  "application/json",
	},
	model.WebhookServiceNtfy: {
		bodyTemplate: `{{.Title}} is now {{.Status}}`,
		contentType:  "text/plain",
		headers: map[string]string{
			"Title": "binge-os-watch",
			"Tags":  "tv",
		},
	},
	model.WebhookServiceGeneric: {
		bodyTemplate: `{"event":"{{.Event}}","title":{{json .Title}},"media_type":"{{.MediaType}}","status":"{{.Status}}","media_id":"{{.MediaID}}"}`,
		contentType:  "application/json",
	},
}

// templateData is the variable set available in webhook templates.
type templateData struct {
	Event     string
	Title     string
	MediaType string
	Status    string
	MediaID   string
}

// renderBody renders the body template using a json-safe escape function.
func renderBody(tmplStr string, data templateData) (string, error) {
	tmpl, err := template.New("body").Funcs(template.FuncMap{
		"json": func(v any) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
	}).Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// buildRequest determines the body, content type, and headers for a webhook.
func buildRequest(wh *model.Webhook, data templateData) (body string, contentType string, headers map[string]string, err error) {
	headers = map[string]string{}

	if wh.Service == model.WebhookServiceCustom {
		bodyTmpl := wh.BodyTemplate
		if bodyTmpl == "" {
			bodyTmpl = presets[model.WebhookServiceGeneric].bodyTemplate
		}
		body, err = renderBody(bodyTmpl, data)
		if err != nil {
			return "", "", nil, err
		}
		contentType = "application/json"
		// Parse custom headers JSON.
		if wh.Headers != "" {
			var custom map[string]string
			if jerr := json.Unmarshal([]byte(wh.Headers), &custom); jerr == nil {
				for k, v := range custom {
					headers[k] = v
					if k == "Content-Type" {
						contentType = v
					}
				}
			}
		}
		return body, contentType, headers, nil
	}

	preset, ok := presets[wh.Service]
	if !ok {
		preset = presets[model.WebhookServiceGeneric]
	}
	body, err = renderBody(preset.bodyTemplate, data)
	if err != nil {
		return "", "", nil, err
	}
	contentType = preset.contentType
	for k, v := range preset.headers {
		headers[k] = v
	}
	return body, contentType, headers, nil
}

// Dispatch fires webhook POSTs for the given event. Fire-and-forget.
func (s *WebhookServiceImpl) Dispatch(ctx context.Context, userID, event string, payload any) {
	hooks, err := s.webhooks.ListByEvent(ctx, userID, event)
	if err != nil || len(hooks) == 0 {
		return
	}

	data := payloadToTemplateData(event, payload)

	for _, h := range hooks {
		go s.dispatchOne(h, data)
	}
}

func (s *WebhookServiceImpl) dispatchOne(wh model.Webhook, data templateData) {
	delivery := &model.WebhookDelivery{
		WebhookID: wh.ID,
		Event:     data.Event,
	}

	body, contentType, headers, err := buildRequest(&wh, data)
	if err != nil {
		delivery.Error = "template error: " + err.Error()
		s.webhooks.CreateDelivery(context.Background(), delivery)
		return
	}

	reqCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, wh.URL, bytes.NewReader([]byte(body)))
	if err != nil {
		delivery.Error = err.Error()
		s.webhooks.CreateDelivery(context.Background(), delivery)
		return
	}
	req.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		delivery.Error = err.Error()
		s.webhooks.CreateDelivery(context.Background(), delivery)
		slog.Error("webhook dispatch failed", "url", wh.URL, "error", err)
		return
	}
	resp.Body.Close()
	delivery.StatusCode = resp.StatusCode
	s.webhooks.CreateDelivery(context.Background(), delivery)
}

// DispatchTest sends a test webhook to the given webhook config and records the delivery.
func (s *WebhookServiceImpl) DispatchTest(ctx context.Context, wh model.Webhook) {
	s.dispatchOne(wh, templateData{
		Event:     "test",
		Title:     "Test webhook from binge-os-watch",
		MediaType: "movie",
		Status:    "watched",
		MediaID:   "test-id",
	})
}

// payloadToTemplateData converts a Dispatch payload to template variables.
func payloadToTemplateData(event string, payload any) templateData {
	d := templateData{Event: event}
	if m, ok := payload.(map[string]any); ok {
		if v, ok := m["title"].(string); ok {
			d.Title = v
		}
		if v, ok := m["media_type"]; ok {
			d.MediaType = fmt.Sprintf("%v", v)
		}
		if v, ok := m["status"]; ok {
			d.Status = fmt.Sprintf("%v", v)
		}
		if v, ok := m["media_id"].(string); ok {
			d.MediaID = v
		}
	}
	return d
}
