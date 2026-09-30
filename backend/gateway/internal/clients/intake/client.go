package intakeclient

import (
	"bytes"
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
//           INTAKE CLIENT INTERFACE        //
//==========================================//

// Client is the interface the gateway uses to call the Intake service.
//
// The concrete implementation below uses plain HTTP/JSON.
// To swap in gRPC: implement this interface with a gRPC client, drop it in at
// the DI site in main.go — the gateway handlers never know the difference.
type Client interface {
	SubmitAudit(ctx context.Context, req SubmitAuditRequest) (SubmitAuditResponse, error)
}

type SubmitAuditRequest struct {
	URL      string    `json:"url"`
	TenantID uuid.UUID `json:"tenant_id"`
}

type SubmitAuditResponse struct {
	AuditID uuid.UUID `json:"audit_id"`
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
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // intake can take up to ~10s for reachability
		},
		logger: logger,
	}
}

// SubmitAudit implements [Client].
// POST /audits on the intake service, returns the new audit ID.
func (c *httpClient) SubmitAudit(ctx context.Context, req SubmitAuditRequest) (SubmitAuditResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return SubmitAuditResponse{}, fmt.Errorf("marshal intake request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audits", bytes.NewReader(body))
	if err != nil {
		return SubmitAuditResponse{}, fmt.Errorf("build intake request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return SubmitAuditResponse{}, fmt.Errorf("intake unreachable: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		// Surface the intake error message to the caller so the gateway can
		// forward it to the client with an appropriate status code.
		return SubmitAuditResponse{}, &IntakeError{
			StatusCode: resp.StatusCode,
			Body:       string(raw),
		}
	}

	// Intake response shape: {"data":{"audit_id":"..."},"request_id":"..."}
	var envelope struct {
		Data struct {
			AuditID uuid.UUID `json:"audit_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return SubmitAuditResponse{}, fmt.Errorf("decode intake response: %w", err)
	}

	return SubmitAuditResponse{AuditID: envelope.Data.AuditID}, nil
}

//==========================================//
//              TYPED ERRORS                //
//==========================================//

// IntakeError is returned when the intake service itself rejects the request.
// The gateway maps the HTTP status and body back to a clean error for the client.
type IntakeError struct {
	StatusCode int
	Body       string
}

func (e *IntakeError) Error() string {
	return fmt.Sprintf("intake rejected request (status %d): %s", e.StatusCode, e.Body)
}
