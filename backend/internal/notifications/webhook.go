package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/yourorg/secret-manager/pkg/logger"
)

// WebhookClient handles sending notifications to external webhooks
type WebhookClient struct {
	webhookURL string
	httpClient *http.Client
}

// NewWebhookClient creates a new webhook client
func NewWebhookClient(webhookURL string) *WebhookClient {
	if webhookURL == "" {
		return nil
	}

	return &WebhookClient{
		webhookURL: webhookURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// DriftNotification represents a drift detection notification
type DriftNotification struct {
	Namespace  string    `json:"namespace"`
	SecretName string    `json:"secret_name"`
	DriftType  string    `json:"drift_type"`
	DetectedAt time.Time `json:"detected_at"`
	Message    string    `json:"message"`
}

// SendDriftNotification sends a drift notification to the webhook
func (w *WebhookClient) SendDriftNotification(notification DriftNotification) error {
	if w == nil {
		return nil // Webhook not configured
	}

	// Format for Slack/Discord compatibility
	payload := map[string]interface{}{
		"text": notification.Message,
		"attachments": []map[string]interface{}{
			{
				"color": "warning",
				"fields": []map[string]interface{}{
					{"title": "Namespace", "value": notification.Namespace, "short": true},
					{"title": "Secret", "value": notification.SecretName, "short": true},
					{"title": "Type", "value": notification.DriftType, "short": true},
					{"title": "Detected", "value": notification.DetectedAt.Format(time.RFC3339), "short": true},
				},
			},
		},
	}

	return w.sendWithRetry(payload, 3)
}

func (w *WebhookClient) sendWithRetry(payload interface{}, maxRetries int) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := w.httpClient.Post(w.webhookURL, "application/json", bytes.NewBuffer(jsonData))
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resp.Body.Close()
			logger.Info("Webhook notification sent successfully", "attempt", attempt)
			return nil
		}

		if resp != nil {
			resp.Body.Close()
			lastErr = fmt.Errorf("webhook returned status %d", resp.StatusCode)
		} else {
			lastErr = err
		}

		if attempt < maxRetries {
			backoff := time.Duration(attempt*attempt) * time.Second
			logger.Warn("Webhook notification failed, retrying", "attempt", attempt, "backoff", backoff, "error", lastErr)
			time.Sleep(backoff)
		}
	}

	return fmt.Errorf("webhook failed after %d attempts: %w", maxRetries, lastErr)
}
