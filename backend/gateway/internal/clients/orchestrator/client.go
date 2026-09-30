package orchestratorclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

//==========================================//
//         ORCHESTRATOR CLIENT INTERFACE    //
//==========================================//

// Client is the interface the gateway uses to call the Orchestrator service.
//
// Same contract as the intake client: swap the HTTP impl for gRPC in one place,
// gateway handlers are untouched.
type Client interface {
	GetAuditStatus(ctx context.Context, auditID uuid.UUID) (AuditStatusResponse, error)
}

type AuditStatusResponse struct {
	ID            uuid.UUID  `json:"id"`
	TenantID      uuid.UUID  `json:"tenant_id"`
	NormalizedURL string     `json:"normalized_url"`
	Domain        string     `json:"domain"`
	Status        string     `json:"status"`
	StepStartedAt *time.Time `json:"step_started_at,omitempty"`
	StepAttempts  int        `json:"step_attempts"`
	SubmittedAt   time.Time  `json:"submitted_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

//==========================================//
//           HTTP IMPLEMENTATION            //
//==========================================//

type httpClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

func NewHTTPClient(baseURL string, logger *zap.Logger) Client {
	return &httpClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

// GetAuditStatus implements [Client].
// GET /internal/audits/:id/status on the orchestrator.
func (c *httpClient) GetAuditStatus(ctx context.Context, auditID uuid.UUID) (AuditStatusResponse, error) {
	url := fmt.Sprintf("%s/internal/audits/%s/status", c.baseURL, auditID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return AuditStatusResponse{}, fmt.Errorf("build orchestrator request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return AuditStatusResponse{}, fmt.Errorf("orchestrator unreachable: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return AuditStatusResponse{}, ErrNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return AuditStatusResponse{}, fmt.Errorf("orchestrator error (status %d): %s", resp.StatusCode, string(raw))
	}

	// Orchestrator response shape: {"data":{...},"request_id":"..."}
	var envelope struct {
		Data AuditStatusResponse `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return AuditStatusResponse{}, fmt.Errorf("decode orchestrator response: %w", err)
	}

	return envelope.Data, nil
}

//==========================================//
//              SENTINEL ERRORS             //
//==========================================//

var ErrNotFound = fmt.Errorf("audit not found")
