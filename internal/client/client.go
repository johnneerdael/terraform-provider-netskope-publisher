// Package client is a thin HTTP wrapper for the Netskope NPA publisher endpoints
// needed by this provider. It is intentionally narrow — only the calls the two
// resources need.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to /api/v2/infrastructure/publishers on a Netskope tenant.
type Client struct {
	BaseURL    string // e.g. https://tenant.goskope.com
	APIToken   string
	HTTPClient *http.Client
}

// New constructs a client. baseURL trailing slash is normalized away.
func New(baseURL, apiToken string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIToken:   apiToken,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Publisher mirrors the NPA API publisher object (subset we use).
type Publisher struct {
	ID            int64  `json:"id,omitempty"`
	PublisherID   int64  `json:"publisher_id,omitempty"`
	Name          string `json:"name,omitempty"`
	PublisherName string `json:"publisher_name,omitempty"`
	CommonName    string `json:"common_name,omitempty"`
	Registered    bool   `json:"registered,omitempty"`
	Status        string `json:"status,omitempty"`
}

// ID resolves whichever id field the API populated.
func (p *Publisher) ResolvedID() int64 {
	if p.PublisherID != 0 {
		return p.PublisherID
	}
	return p.ID
}

// ResolvedName resolves whichever name field the API populated.
func (p *Publisher) ResolvedName() string {
	if p.PublisherName != "" {
		return p.PublisherName
	}
	return p.Name
}

type listResponse struct {
	Status string `json:"status"`
	Data   struct {
		Publishers []Publisher `json:"publishers"`
	} `json:"data"`
}

type singleResponse struct {
	Status string    `json:"status"`
	Data   Publisher `json:"data"`
}

type tokenResponse struct {
	Status string `json:"status"`
	Data   struct {
		Token string `json:"token"`
	} `json:"data"`
}

type errorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// do is the common request executor.
func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, []byte, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bodyReader)
	if err != nil {
		return nil, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Netskope-Api-Token", c.APIToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, fmt.Errorf("read response: %w", err)
	}
	return resp, respBody, nil
}

// ListPublishers GETs /api/v2/infrastructure/publishers and returns the list.
func (c *Client) ListPublishers(ctx context.Context) ([]Publisher, error) {
	resp, body, err := c.do(ctx, http.MethodGet, "/api/v2/infrastructure/publishers", nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list publishers: status %d: %s", resp.StatusCode, string(body))
	}
	var lr listResponse
	if err := json.Unmarshal(body, &lr); err != nil {
		return nil, fmt.Errorf("decode list response: %w (body=%q)", err, string(body))
	}
	return lr.Data.Publishers, nil
}

// FindPublisherByName scans the list for an exact-name match. Returns nil if not found.
func (c *Client) FindPublisherByName(ctx context.Context, name string) (*Publisher, error) {
	pubs, err := c.ListPublishers(ctx)
	if err != nil {
		return nil, err
	}
	for i := range pubs {
		if pubs[i].ResolvedName() == name {
			return &pubs[i], nil
		}
	}
	return nil, nil
}

// GetPublisher GETs /api/v2/infrastructure/publishers/{id}.
func (c *Client) GetPublisher(ctx context.Context, id int64) (*Publisher, error) {
	resp, body, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/v2/infrastructure/publishers/%d", id), nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get publisher %d: status %d: %s", id, resp.StatusCode, string(body))
	}
	var sr singleResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("decode get response: %w (body=%q)", err, string(body))
	}
	return &sr.Data, nil
}

// CreatePublisher POSTs /api/v2/infrastructure/publishers with {"name": name}.
// Returns the created publisher (with the new id).
func (c *Client) CreatePublisher(ctx context.Context, name string) (*Publisher, error) {
	resp, body, err := c.do(ctx, http.MethodPost, "/api/v2/infrastructure/publishers", map[string]string{"name": name})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("create publisher %q: status %d: %s", name, resp.StatusCode, string(body))
	}
	var sr singleResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("decode create response: %w (body=%q)", err, string(body))
	}
	return &sr.Data, nil
}

// UpdatePublisherName PATCHes the publisher to change its name. Returns the updated publisher.
func (c *Client) UpdatePublisherName(ctx context.Context, id int64, newName string) (*Publisher, error) {
	resp, body, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/api/v2/infrastructure/publishers/%d", id), map[string]string{"name": newName})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("update publisher %d: status %d: %s", id, resp.StatusCode, string(body))
	}
	var sr singleResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("decode update response: %w (body=%q)", err, string(body))
	}
	return &sr.Data, nil
}

// DeletePublisher DELETEs /api/v2/infrastructure/publishers/{id}.
func (c *Client) DeletePublisher(ctx context.Context, id int64) error {
	resp, body, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/api/v2/infrastructure/publishers/%d", id), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("delete publisher %d: status %d: %s", id, resp.StatusCode, string(body))
	}
	return nil
}

// IssueRegistrationToken POSTs /api/v2/infrastructure/publishers/{id}/registration_token.
func (c *Client) IssueRegistrationToken(ctx context.Context, publisherID int64) (string, error) {
	resp, body, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/api/v2/infrastructure/publishers/%d/registration_token", publisherID), nil)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("issue token for publisher %d: status %d: %s", publisherID, resp.StatusCode, string(body))
	}
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("decode token response: %w (body=%q)", err, string(body))
	}
	if tr.Data.Token == "" {
		return "", fmt.Errorf("issue token for publisher %d: empty token in response", publisherID)
	}
	return tr.Data.Token, nil
}
