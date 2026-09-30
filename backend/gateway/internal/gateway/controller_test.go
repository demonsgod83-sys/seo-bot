package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	intakeclient "github.com/demonsgod83-sys/seo-bot/gateway/internal/clients/intake"
	orchestratorclient "github.com/demonsgod83-sys/seo-bot/gateway/internal/clients/orchestrator"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

//==========================================//
//              MOCK CLIENTS                //
//==========================================//

type mockIntake struct {
	fn func(ctx context.Context, req intakeclient.SubmitAuditRequest) (intakeclient.SubmitAuditResponse, error)
}

func (m *mockIntake) SubmitAudit(ctx context.Context, req intakeclient.SubmitAuditRequest) (intakeclient.SubmitAuditResponse, error) {
	return m.fn(ctx, req)
}

type mockOrchestrator struct {
	fn func(ctx context.Context, auditID uuid.UUID) (orchestratorclient.AuditStatusResponse, error)
}

func (m *mockOrchestrator) GetAuditStatus(ctx context.Context, auditID uuid.UUID) (orchestratorclient.AuditStatusResponse, error) {
	return m.fn(ctx, auditID)
}

//==========================================//
//              TEST HELPERS                //
//==========================================//

func makeHandler(intake intakeclient.Client, orch orchestratorclient.Client) *Handler {
	logger, _ := zap.NewDevelopment()
	return NewHandler(intake, orch, logger)
}

// ginTestContext creates a test gin context with a pre-set tenant ID and request_id.
func ginTestContext(method, path string, body []byte, tenantID uuid.UUID) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("tenant_id", tenantID)
	c.Set("request_id", "test-request-id")
	return c, w
}

func decodeResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v\nbody: %s", err, w.Body.String())
	}
	return result
}

//==========================================//
//           SUBMIT AUDIT TESTS             //
//==========================================//

func TestSubmitAudit_MissingURL(t *testing.T) {
	h := makeHandler(nil, nil)
	body := []byte(`{}`)
	c, w := ginTestContext(http.MethodPost, "/v1/audits", body, uuid.New())

	h.SubmitAudit(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	resp := decodeResponse(t, w)
	errBlock := resp["error"].(map[string]interface{})
	if errBlock["code"] != "INVALID_REQUEST" {
		t.Errorf("expected code INVALID_REQUEST, got %v", errBlock["code"])
	}
}

func TestSubmitAudit_EmptyURL(t *testing.T) {
	h := makeHandler(nil, nil)
	body := []byte(`{"url": "   "}`)
	c, w := ginTestContext(http.MethodPost, "/v1/audits", body, uuid.New())

	h.SubmitAudit(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	resp := decodeResponse(t, w)
	errBlock := resp["error"].(map[string]interface{})
	if errBlock["code"] != "MISSING_FIELD" {
		t.Errorf("expected code MISSING_FIELD, got %v", errBlock["code"])
	}
}

func TestSubmitAudit_MalformedJSON(t *testing.T) {
	h := makeHandler(nil, nil)
	body := []byte(`{not json}`)
	c, w := ginTestContext(http.MethodPost, "/v1/audits", body, uuid.New())

	h.SubmitAudit(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSubmitAudit_IntakeRejectsURL(t *testing.T) {
	expectedAuditID := uuid.New()
	_ = expectedAuditID // suppress unused warning

	mock := &mockIntake{fn: func(ctx context.Context, req intakeclient.SubmitAuditRequest) (intakeclient.SubmitAuditResponse, error) {
		return intakeclient.SubmitAuditResponse{}, &intakeclient.IntakeError{
			StatusCode: http.StatusBadRequest,
			Body:       "URL validation failed: site is unreachable",
		}
	}}

	h := makeHandler(mock, nil)
	body := []byte(`{"url": "https://notarealdomain12345.invalid"}`)
	c, w := ginTestContext(http.MethodPost, "/v1/audits", body, uuid.New())

	h.SubmitAudit(c)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
	resp := decodeResponse(t, w)
	errBlock := resp["error"].(map[string]interface{})
	if errBlock["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected code VALIDATION_ERROR, got %v", errBlock["code"])
	}
}

func TestSubmitAudit_Success(t *testing.T) {
	expectedAuditID := uuid.New()
	tenantID := uuid.New()

	mock := &mockIntake{fn: func(ctx context.Context, req intakeclient.SubmitAuditRequest) (intakeclient.SubmitAuditResponse, error) {
		// Verify the tenant_id from context was injected correctly
		if req.TenantID != tenantID {
			t.Errorf("expected tenantID %s, got %s", tenantID, req.TenantID)
		}
		if req.URL != "https://example.com" {
			t.Errorf("expected URL https://example.com, got %s", req.URL)
		}
		return intakeclient.SubmitAuditResponse{AuditID: expectedAuditID}, nil
	}}

	h := makeHandler(mock, nil)
	body := []byte(`{"url": "https://example.com"}`)
	c, w := ginTestContext(http.MethodPost, "/v1/audits", body, tenantID)

	h.SubmitAudit(c)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d\nbody: %s", w.Code, w.Body.String())
	}

	resp := decodeResponse(t, w)
	data := resp["data"].(map[string]interface{})
	if data["audit_id"] != expectedAuditID.String() {
		t.Errorf("expected audit_id %s, got %v", expectedAuditID, data["audit_id"])
	}
}

//==========================================//
//           GET AUDIT STATUS TESTS         //
//==========================================//

func TestGetAuditStatus_InvalidID(t *testing.T) {
	h := makeHandler(nil, nil)
	c, w := ginTestContext(http.MethodGet, "/v1/audits/not-a-uuid", nil, uuid.New())
	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}

	h.GetAuditStatus(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	resp := decodeResponse(t, w)
	errBlock := resp["error"].(map[string]interface{})
	if errBlock["code"] != "INVALID_ID" {
		t.Errorf("expected code INVALID_ID, got %v", errBlock["code"])
	}
}

func TestGetAuditStatus_NotFound(t *testing.T) {
	mock := &mockOrchestrator{fn: func(ctx context.Context, auditID uuid.UUID) (orchestratorclient.AuditStatusResponse, error) {
		return orchestratorclient.AuditStatusResponse{}, orchestratorclient.ErrNotFound
	}}

	h := makeHandler(nil, mock)
	auditID := uuid.New()
	c, w := ginTestContext(http.MethodGet, "/v1/audits/"+auditID.String(), nil, uuid.New())
	c.Params = gin.Params{{Key: "id", Value: auditID.String()}}

	h.GetAuditStatus(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetAuditStatus_TenantIsolation(t *testing.T) {
	auditOwnerTenantID := uuid.New()
	requestingTenantID := uuid.New() // different tenant

	mock := &mockOrchestrator{fn: func(ctx context.Context, auditID uuid.UUID) (orchestratorclient.AuditStatusResponse, error) {
		return orchestratorclient.AuditStatusResponse{
			TenantID: auditOwnerTenantID, // belongs to a different tenant
			Status:   "crawling",
		}, nil
	}}

	h := makeHandler(nil, mock)
	auditID := uuid.New()
	c, w := ginTestContext(http.MethodGet, "/v1/audits/"+auditID.String(), nil, requestingTenantID)
	c.Params = gin.Params{{Key: "id", Value: auditID.String()}}

	h.GetAuditStatus(c)

	// Must return 404, NOT 403 — don't reveal the audit's existence to wrong tenants
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for cross-tenant access, got %d\nbody: %s", w.Code, w.Body.String())
	}
}

func TestGetAuditStatus_Success(t *testing.T) {
	tenantID := uuid.New()
	auditID := uuid.New()

	mock := &mockOrchestrator{fn: func(ctx context.Context, id uuid.UUID) (orchestratorclient.AuditStatusResponse, error) {
		return orchestratorclient.AuditStatusResponse{
			ID:       auditID,
			TenantID: tenantID,
			Domain:   "example.com",
			Status:   "crawling",
		}, nil
	}}

	h := makeHandler(nil, mock)
	c, w := ginTestContext(http.MethodGet, "/v1/audits/"+auditID.String(), nil, tenantID)
	c.Params = gin.Params{{Key: "id", Value: auditID.String()}}

	h.GetAuditStatus(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d\nbody: %s", w.Code, w.Body.String())
	}

	resp := decodeResponse(t, w)
	data := resp["data"].(map[string]interface{})
	if data["status"] != "crawling" {
		t.Errorf("expected status crawling, got %v", data["status"])
	}
	if data["domain"] != "example.com" {
		t.Errorf("expected domain example.com, got %v", data["domain"])
	}
}
