package edgeclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
)

// HTTPClient is an HTTP-based implementation of EdgeDeviceClient.
type HTTPClient struct {
	client *http.Client
}

// NewHTTPClient creates a new HTTPClient with configurable timeout.
func NewHTTPClient(timeout time.Duration) *HTTPClient {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &HTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// StatusCheck calls GET {baseURL}/status and returns the check result.
func (c *HTTPClient) StatusCheck(ctx context.Context, baseURL string) (*edge_devices.CheckResult, error) {
	return c.callEndpoint(ctx, baseURL+"/status")
}

// HealthCheck calls GET {baseURL}/health and returns the check result.
func (c *HTTPClient) HealthCheck(ctx context.Context, baseURL string) (*edge_devices.CheckResult, error) {
	return c.callEndpoint(ctx, baseURL+"/health")
}

// GetTelemetry calls GET {baseURL}/telemetry and returns the snapshot.
func (c *HTTPClient) GetTelemetry(ctx context.Context, baseURL string) (*edge_devices.TelemetrySnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/telemetry", nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transport: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("device returned status %d", resp.StatusCode)
	}

	var snapshot edge_devices.TelemetrySnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return nil, fmt.Errorf("parse device response: %w", err)
	}

	if snapshot.CapturedAt.IsZero() {
		snapshot.CapturedAt = time.Now()
	}
	return &snapshot, nil
}

// callEndpoint is a helper to call check endpoints (/status, /health).
func (c *HTTPClient) callEndpoint(ctx context.Context, url string) (*edge_devices.CheckResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transport: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &edge_devices.CheckResult{
			CheckedAt:     time.Now(),
			OverallStatus: "ERROR",
			Summary:       stringPtr("non-2xx status from device"),
			Details:       make(map[string]interface{}),
		}, nil
	}

	var result edge_devices.CheckResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse device response: %w", err)
	}

	if result.CheckedAt.IsZero() {
		result.CheckedAt = time.Now()
	}
	if result.Details == nil {
		result.Details = make(map[string]interface{})
	}
	return &result, nil
}

func stringPtr(s string) *string {
	return &s
}
