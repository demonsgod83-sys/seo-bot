package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type mockRepository struct {
	tenantCfg   *TenantNotificationConfig
	deliveries  map[string]*NotificationDelivery
	auditScore  *AuditScore
	issues      []PrioritizedIssue
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		deliveries: make(map[string]*NotificationDelivery),
	}
}

func (m *mockRepository) GetTenantConfig(ctx context.Context, tenantID uuid.UUID) (*TenantNotificationConfig, error) {
	return m.tenantCfg, nil
}

func (m *mockRepository) SaveTenantConfig(ctx context.Context, cfg *TenantNotificationConfig) error {
	m.tenantCfg = cfg
	return nil
}

func (m *mockRepository) FindDelivery(ctx context.Context, auditID uuid.UUID, channel string) (*NotificationDelivery, error) {
	key := fmt.Sprintf("%s:%s", auditID, channel)
	if d, ok := m.deliveries[key]; ok {
		return d, nil
	}
	return nil, nil
}

func (m *mockRepository) SaveDelivery(ctx context.Context, delivery *NotificationDelivery) error {
	key := fmt.Sprintf("%s:%s", delivery.AuditID, delivery.Channel)
	m.deliveries[key] = delivery
	return nil
}

func (m *mockRepository) UpdateDelivery(ctx context.Context, delivery *NotificationDelivery) error {
	key := fmt.Sprintf("%s:%s", delivery.AuditID, delivery.Channel)
	m.deliveries[key] = delivery
	return nil
}

func (m *mockRepository) GetDeliveriesByAuditID(ctx context.Context, auditID uuid.UUID) ([]NotificationDelivery, error) {
	var list []NotificationDelivery
	for _, d := range m.deliveries {
		if d.AuditID == auditID {
			list = append(list, *d)
		}
	}
	return list, nil
}

func (m *mockRepository) GetAuditScore(ctx context.Context, auditID uuid.UUID) (*AuditScore, error) {
	return m.auditScore, nil
}

func (m *mockRepository) GetTopPrioritizedIssues(ctx context.Context, auditID uuid.UUID, limit int) ([]PrioritizedIssue, error) {
	return m.issues, nil
}

type mockEmailSender struct {
	sentCount int
	lastTo    string
}

func (m *mockEmailSender) SendAuditCompletedEmail(
	to, domain string,
	overallScore, technicalScore, onpageScore, contentScore int,
	topIssues []PrioritizedIssue,
	shareURL string,
) error {
	m.sentCount++
	m.lastTo = to
	return nil
}

func (m *mockEmailSender) SendAuditFailedEmail(to, domain, reason string) error {
	m.sentCount++
	m.lastTo = to
	return nil
}

func TestWebhookSigning(t *testing.T) {
	secret := "test_secret_key_12345"
	timestamp := "1727410000"
	payload := []byte(`{"event":"audit.completed","domain":"example.com"}`)

	sig := generateSignature(payload, timestamp, secret)

	// Verify manually
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	if sig != expected {
		t.Errorf("signature mismatch: expected %s, got %s", expected, sig)
	}
}

func TestWebhookSender_Success(t *testing.T) {
	secret := "secret_key_abc"
	received := false

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("X-Audit-Event") != "audit.completed" {
			t.Errorf("expected X-Audit-Event audit.completed, got %s", r.Header.Get("X-Audit-Event"))
		}
		sigHeader := r.Header.Get("X-Signature-SHA256")
		if sigHeader == "" {
			t.Errorf("missing signature header")
		}

		received = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	logger := zap.NewNop()
	sender := NewWebhookSender(logger)

	payload := WebhookPayload{
		Event:        "audit.completed",
		AuditID:      uuid.New(),
		TenantID:     uuid.New(),
		Domain:       "example.com",
		Status:       "completed",
		OverallScore: 88,
		Timestamp:    time.Now().UTC(),
	}

	err := sender.SendWebhook(context.Background(), ts.URL, secret, payload)
	if err != nil {
		t.Fatalf("webhook failed: %v", err)
	}

	if !received {
		t.Errorf("expected server to receive webhook")
	}
}

func TestService_IdempotencyAndEmailDelivery(t *testing.T) {
	mockRepo := newMockRepository()
	emailSender := &mockEmailSender{}
	webhookSender := NewWebhookSender(zap.NewNop())
	logger := zap.NewNop()

	svc := &service{
		repository:         mockRepo,
		emailSender:        emailSender,
		webhookSender:      webhookSender,
		js:                 nil,
		defaultNotifyEmail: "client@example.com",
		logger:             logger,
	}

	auditID := uuid.New()
	tenantID := uuid.New()

	mockRepo.auditScore = &AuditScore{
		AuditID:        auditID,
		TenantID:       tenantID,
		OverallScore:   85,
		TechnicalScore: 90,
		OnPageScore:    80,
		ContentScore:   85,
	}

	mockRepo.issues = []PrioritizedIssue{
		{Title: "Missing Image Alt", Severity: "warning", EffortTier: "low", PriorityScore: 50.0, AffectedPagesCount: 5},
	}

	// First delivery
	status1 := svc.deliverEmailWithRetry(
		context.Background(),
		auditID, tenantID,
		"client@example.com",
		"example.com",
		85, 90, 80, 85,
		mockRepo.issues,
		"http://localhost:8105/public/reports/abc",
	)

	if status1 != "delivered" {
		t.Errorf("expected first delivery status 'delivered', got %s", status1)
	}
	if emailSender.sentCount != 1 {
		t.Errorf("expected sentCount 1, got %d", emailSender.sentCount)
	}

	// Second delivery (duplicate event) -> should be idempotent and NOT call sender again!
	status2 := svc.deliverEmailWithRetry(
		context.Background(),
		auditID, tenantID,
		"client@example.com",
		"example.com",
		85, 90, 80, 85,
		mockRepo.issues,
		"http://localhost:8105/public/reports/abc",
	)

	if status2 != "delivered" {
		t.Errorf("expected second delivery status 'delivered', got %s", status2)
	}
	if emailSender.sentCount != 1 {
		t.Errorf("expected sentCount to still be 1 after second call (idempotency check), got %d", emailSender.sentCount)
	}
}

func TestService_DeliveryFailureHandling(t *testing.T) {
	mockRepo := newMockRepository()
	logger := zap.NewNop()

	webhookSender := NewWebhookSender(logger)
	emailSender := &mockEmailSender{}

	svc := &service{
		repository:         mockRepo,
		emailSender:        emailSender,
		webhookSender:      webhookSender,
		js:                 nil,
		defaultNotifyEmail: "client@example.com",
		logger:             logger,
	}

	auditID := uuid.New()
	tenantID := uuid.New()

	payload := WebhookPayload{
		Event:     "audit.completed",
		AuditID:   auditID,
		TenantID:  tenantID,
		Domain:    "example.com",
		Status:    "completed",
		Timestamp: time.Now().UTC(),
	}

	// Use an invalid unreachable URL to simulate delivery failure
	status := svc.deliverWebhookWithRetry(context.Background(), auditID, tenantID, "http://127.0.0.1:54321/invalid", "secret", payload)

	if status != "failed" {
		t.Errorf("expected delivery status 'failed', got %s", status)
	}

	deliveries, _ := mockRepo.GetDeliveriesByAuditID(context.Background(), auditID)
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery record in repository")
	}

	if deliveries[0].Status != "failed" {
		t.Errorf("expected delivery record status 'failed', got %s", deliveries[0].Status)
	}
	if deliveries[0].LastError == "" {
		t.Errorf("expected LastError to be populated")
	}
}
