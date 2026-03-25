// Package cloudflare provides Cloudflare event integration for the sync service.
package cloudflare

import (
	"context"
	"fmt"
	"log"
	"time"
)

// EventType represents the type of Cloudflare event
type EventType string

const (
	// Audit log events
	EventAuditLog EventType = "audit_log"

	// Notification/Alert events
	EventAlert EventType = "alert"

	// Logpush events
	EventLogpush EventType = "logpush"

	// Pages deploy events
	EventPagesDeploy EventType = "pages_deploy"

	// Workers deploy events
	EventWorkersDeploy EventType = "workers_deploy"

	// Tunnel events
	EventTunnel EventType = "tunnel"
)

// Event represents a normalized Cloudflare event
type Event struct {
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	AccountID string                 `json:"account_id,omitempty"`
	ZoneID    string                 `json:"zone_id,omitempty"`
	Action    string                 `json:"action"`
	Resource  string                 `json:"resource"`
	Actor     string                 `json:"actor,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Raw       interface{}            `json:"raw,omitempty"`
}

// EventHandler is a callback for processing events
type EventHandler func(ctx context.Context, event Event) error

// Client is the main Cloudflare client for sync integration
type Client struct {
	apiToken  string
	accountID string
	handlers  map[EventType][]EventHandler
}

// Config holds configuration for the Cloudflare client
type Config struct {
	// APIToken is the Cloudflare API token
	// Required scopes: Account Audit Logs:Read, Notifications:Read
	APIToken string

	// AccountID is the Cloudflare account ID
	AccountID string

	// PollInterval for audit log polling (default: 1 minute)
	PollInterval time.Duration
}

// NewClient creates a new Cloudflare client
func NewClient(cfg Config) (*Client, error) {
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("API token is required")
	}
	if cfg.AccountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}

	return &Client{
		apiToken:  cfg.APIToken,
		accountID: cfg.AccountID,
		handlers:  make(map[EventType][]EventHandler),
	}, nil
}

// On registers an event handler for a specific event type
func (c *Client) On(eventType EventType, handler EventHandler) {
	c.handlers[eventType] = append(c.handlers[eventType], handler)
}

// OnAny registers an event handler for all event types
func (c *Client) OnAny(handler EventHandler) {
	for _, et := range []EventType{EventAuditLog, EventAlert, EventLogpush, EventPagesDeploy, EventWorkersDeploy, EventTunnel} {
		c.On(et, handler)
	}
}

// emit sends an event to all registered handlers
func (c *Client) emit(ctx context.Context, event Event) {
	handlers := c.handlers[event.Type]
	for _, h := range handlers {
		if err := h(ctx, event); err != nil {
			log.Printf("cloudflare: handler error for %s: %v", event.Type, err)
		}
	}
}

// GetAccountID returns the configured account ID
func (c *Client) GetAccountID() string {
	return c.accountID
}
