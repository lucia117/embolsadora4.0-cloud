package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AdminClient interacts with the Supabase Admin REST API.
type AdminClient interface {
	// InviteUserByEmail sends an invitation email via Supabase.
	// redirectTo should be the full frontend callback URL including tenantId.
	InviteUserByEmail(ctx context.Context, email, redirectTo string) error

	// SendPasswordResetEmail sends a password reset email via Supabase generate-link API.
	SendPasswordResetEmail(ctx context.Context, userEmail string) error
}

type adminClient struct {
	supabaseURL    string
	serviceRoleKey string
	httpClient     *http.Client
}

// NewAdminClient creates a new Supabase Admin API client.
func NewAdminClient(supabaseURL, serviceRoleKey string) AdminClient {
	return &adminClient{
		supabaseURL:    supabaseURL,
		serviceRoleKey: serviceRoleKey,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *adminClient) InviteUserByEmail(ctx context.Context, email, redirectTo string) error {
	body, err := json.Marshal(map[string]interface{}{
		"email": email,
		"data": map[string]string{
			"redirect_to": redirectTo,
		},
	})
	if err != nil {
		return fmt.Errorf("marshal invite request: %w", err)
	}

	return c.doWithRetry(ctx, http.MethodPost, "/auth/v1/admin/invite", body)
}

func (c *adminClient) SendPasswordResetEmail(ctx context.Context, userEmail string) error {
	body, err := json.Marshal(map[string]string{
		"type":  "recovery",
		"email": userEmail,
	})
	if err != nil {
		return fmt.Errorf("marshal recovery request: %w", err)
	}

	return c.doWithRetry(ctx, http.MethodPost, "/auth/v1/admin/generate-link", body)
}

// doWithRetry executes the request with 1 retry on 5xx responses.
func (c *adminClient) doWithRetry(ctx context.Context, method, path string, body []byte) error {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.supabaseURL+path, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.serviceRoleKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("supabase admin API request failed: %w", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		// 4xx: do not retry
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("supabase admin API error %d (no retry)", resp.StatusCode)
		}
		// 5xx: retry once
		lastErr = fmt.Errorf("supabase admin API server error %d", resp.StatusCode)
	}
	return lastErr
}
