package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// AuditLogEntry represents a single audit log entry from Cloudflare
type AuditLogEntry struct {
	ID        string    `json:"id"`
	Action    ActionInfo `json:"action"`
	Actor     ActorInfo  `json:"actor"`
	When      time.Time  `json:"when"`
	Resource  ResourceInfo `json:"resource"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ActionInfo describes the action taken
type ActionInfo struct {
	Type   string `json:"type"`
	Result bool   `json:"result"`
}

// ActorInfo describes who performed the action
type ActorInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Type  string `json:"type"`
	IP    string `json:"ip"`
}

// ResourceInfo describes the affected resource
type ResourceInfo struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// AuditLogResponse is the API response structure
type AuditLogResponse struct {
	Success  bool            `json:"success"`
	Errors   []interface{}   `json:"errors"`
	Messages []interface{}   `json:"messages"`
	Result   []AuditLogEntry `json:"result"`
}

// AuditPoller polls Cloudflare audit logs
type AuditPoller struct {
	client       *Client
	interval     time.Duration
	lastSeen     time.Time
	lastSeenLock sync.Mutex
	stopCh       chan struct{}
	httpClient   *http.Client
}

// NewAuditPoller creates a new audit log poller
func NewAuditPoller(client *Client, interval time.Duration) *AuditPoller {
	if interval == 0 {
		interval = 1 * time.Minute
	}

	return &AuditPoller{
		client:     client,
		interval:   interval,
		lastSeen:   time.Now().Add(-5 * time.Minute), // Start 5 minutes back
		stopCh:     make(chan struct{}),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Start begins polling for audit log events
func (p *AuditPoller) Start(ctx context.Context) {
	log.Printf("cloudflare: starting audit log poller (interval: %s)", p.interval)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Initial poll
	p.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

// Stop stops the poller
func (p *AuditPoller) Stop() {
	close(p.stopCh)
}

// poll fetches new audit log entries
func (p *AuditPoller) poll(ctx context.Context) {
	p.lastSeenLock.Lock()
	since := p.lastSeen
	p.lastSeenLock.Unlock()

	entries, err := p.fetchAuditLogs(ctx, since)
	if err != nil {
		log.Printf("cloudflare: audit poll error: %v", err)
		return
	}

	if len(entries) == 0 {
		return
	}

	log.Printf("cloudflare: fetched %d audit log entries", len(entries))

	var latestTime time.Time
	for _, entry := range entries {
		// Convert to normalized event
		event := Event{
			Type:      EventAuditLog,
			Timestamp: entry.When,
			AccountID: p.client.accountID,
			Action:    entry.Action.Type,
			Resource:  fmt.Sprintf("%s/%s", entry.Resource.Type, entry.Resource.ID),
			Actor:     entry.Actor.Email,
			Metadata: map[string]interface{}{
				"action_result": entry.Action.Result,
				"actor_ip":      entry.Actor.IP,
				"actor_type":    entry.Actor.Type,
			},
			Raw: entry,
		}

		p.client.emit(ctx, event)

		if entry.When.After(latestTime) {
			latestTime = entry.When
		}
	}

	// Update last seen time
	if !latestTime.IsZero() {
		p.lastSeenLock.Lock()
		p.lastSeen = latestTime
		p.lastSeenLock.Unlock()
	}
}

// fetchAuditLogs fetches audit logs from the Cloudflare API
func (p *AuditPoller) fetchAuditLogs(ctx context.Context, since time.Time) ([]AuditLogEntry, error) {
	// Build URL with query params
	baseURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/audit_logs", p.client.accountID)

	params := url.Values{}
	params.Set("since", since.UTC().Format(time.RFC3339))
	params.Set("per_page", "100")

	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.client.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp AuditLogResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API returned success=false: %v", apiResp.Errors)
	}

	return apiResp.Result, nil
}

// StartAuditPolling starts polling audit logs on the client
func (c *Client) StartAuditPolling(ctx context.Context, interval time.Duration) *AuditPoller {
	poller := NewAuditPoller(c, interval)
	go poller.Start(ctx)
	return poller
}
