package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"
)

type WebhookSender interface {
	SendWebhook(ctx context.Context, webhookURL, webhookSecret string, payload WebhookPayload) error
}

type webhookSender struct {
	httpClient *http.Client
	logger     *zap.Logger
}

func NewWebhookSender(logger *zap.Logger) WebhookSender {
	return &webhookSender{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

func (w *webhookSender) SendWebhook(ctx context.Context, webhookURL, webhookSecret string, payload WebhookPayload) error {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "SEO-Bot-Webhook/1.0")
	req.Header.Set("X-Audit-Event", payload.Event)
	req.Header.Set("X-Timestamp", timestamp)

	// HMAC-SHA256 Webhook Signature
	if webhookSecret != "" {
		signature := generateSignature(bodyBytes, timestamp, webhookSecret)
		req.Header.Set("X-Signature-SHA256", "sha256="+signature)
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http post to %s: %w", webhookURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook endpoint returned status %d", resp.StatusCode)
	}

	w.logger.Info("webhook delivered successfully",
		zap.String("url", webhookURL),
		zap.Int("status_code", resp.StatusCode),
	)

	return nil
}

func generateSignature(payload []byte, timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
