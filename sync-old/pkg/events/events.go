// Package events provides a unified event bus for GitHub and Cloudflare events.
package events

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Source identifies where an event came from
type Source string

const (
	SourceGitHub     Source = "github"
	SourceCloudflare Source = "cloudflare"
)

// Type identifies the kind of event
type Type string

// GitHub event types
const (
	TypePush         Type = "push"
	TypeRelease      Type = "release"
	TypePullRequest  Type = "pull_request"
	TypeWorkflowRun  Type = "workflow_run"
	TypeCreate       Type = "create" // tag/branch creation
	TypeDelete       Type = "delete" // tag/branch deletion
)

// Cloudflare event types
const (
	TypeAuditLog      Type = "audit_log"
	TypeAlert         Type = "alert"
	TypeLogpush       Type = "logpush"
	TypePagesDeploy   Type = "pages_deploy"
	TypeWorkersDeploy Type = "workers_deploy"
	TypeTunnelHealth  Type = "tunnel_health"
	TypeR2Upload      Type = "r2_upload"
	TypeDNSChange     Type = "dns_change"
)

// Event is a normalized event from any source
type Event struct {
	// Identity
	ID        string    `json:"id"`
	Source    Source    `json:"source"`
	Type      Type      `json:"type"`
	Timestamp time.Time `json:"timestamp"`

	// GitHub-specific
	Repo   string `json:"repo,omitempty"`   // owner/repo
	Ref    string `json:"ref,omitempty"`    // refs/heads/main, refs/tags/v1.0.0
	Sender string `json:"sender,omitempty"` // GitHub username

	// Cloudflare-specific
	AccountID   string `json:"account_id,omitempty"`
	ZoneID      string `json:"zone_id,omitempty"`
	AccountName string `json:"account_name,omitempty"`

	// Common fields
	Action   string                 `json:"action"`   // What happened (created, updated, deleted, etc.)
	Resource string                 `json:"resource"` // What was affected
	Actor    string                 `json:"actor"`    // Who did it (email, username, etc.)
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Raw      interface{}            `json:"-"` // Original event payload
}

// Handler processes events
type Handler func(ctx context.Context, event Event) error

// Filter determines if an event should be processed
type Filter func(event Event) bool

// Bus is the central event bus
type Bus struct {
	handlers []handlerEntry
	mu       sync.RWMutex
}

type handlerEntry struct {
	name    string
	filter  Filter
	handler Handler
}

// NewBus creates a new event bus
func NewBus() *Bus {
	return &Bus{
		handlers: make([]handlerEntry, 0),
	}
}

// On registers a handler for all events
func (b *Bus) On(name string, handler Handler) {
	b.OnFiltered(name, nil, handler)
}

// OnFiltered registers a handler with a filter
func (b *Bus) OnFiltered(name string, filter Filter, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers = append(b.handlers, handlerEntry{
		name:    name,
		filter:  filter,
		handler: handler,
	})
}

// OnSource registers a handler for events from a specific source
func (b *Bus) OnSource(name string, source Source, handler Handler) {
	b.OnFiltered(name, func(e Event) bool {
		return e.Source == source
	}, handler)
}

// OnType registers a handler for events of a specific type
func (b *Bus) OnType(name string, eventType Type, handler Handler) {
	b.OnFiltered(name, func(e Event) bool {
		return e.Type == eventType
	}, handler)
}

// Emit sends an event to all matching handlers
func (b *Bus) Emit(ctx context.Context, event Event) {
	b.mu.RLock()
	handlers := make([]handlerEntry, len(b.handlers))
	copy(handlers, b.handlers)
	b.mu.RUnlock()

	for _, h := range handlers {
		// Check filter
		if h.filter != nil && !h.filter(event) {
			continue
		}

		// Call handler
		if err := h.handler(ctx, event); err != nil {
			log.Printf("events: handler %q error: %v", h.name, err)
		}
	}
}

// String returns a human-readable representation of the event
func (e Event) String() string {
	if e.Repo != "" {
		return fmt.Sprintf("[%s] %s: %s on %s", e.Source, e.Type, e.Action, e.Repo)
	}
	if e.AccountID != "" {
		return fmt.Sprintf("[%s] %s: %s on %s", e.Source, e.Type, e.Action, e.Resource)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Source, e.Type, e.Action)
}

// DefaultHandlers returns commonly used handlers

// LogHandler logs all events
func LogHandler() Handler {
	return func(ctx context.Context, event Event) error {
		log.Printf("EVENT: %s", event.String())
		if event.Actor != "" {
			log.Printf("  Actor: %s", event.Actor)
		}
		if event.Ref != "" {
			log.Printf("  Ref: %s", event.Ref)
		}
		return nil
	}
}

// SubsystemMapper maps events to subsystem names for reload
type SubsystemMapper struct {
	// RepoToSubsystem maps GitHub repos to subsystem names
	// e.g., "nats-io/nats-server" -> "nats"
	RepoToSubsystem map[string]string

	// CFResourceToSubsystem maps CF resources to subsystem names
	// e.g., "pages/docs" -> "docs"
	CFResourceToSubsystem map[string]string
}

// Map returns the subsystem name for an event, or empty string if not mapped
func (m *SubsystemMapper) Map(event Event) string {
	switch event.Source {
	case SourceGitHub:
		if sub, ok := m.RepoToSubsystem[event.Repo]; ok {
			return sub
		}
	case SourceCloudflare:
		if sub, ok := m.CFResourceToSubsystem[event.Resource]; ok {
			return sub
		}
	}
	return ""
}

// ReloadHandler creates a handler that triggers subsystem reloads
func ReloadHandler(mapper *SubsystemMapper, reloadFn func(subsystem string) error) Handler {
	return func(ctx context.Context, event Event) error {
		subsystem := mapper.Map(event)
		if subsystem == "" {
			return nil // No mapping, skip
		}

		log.Printf("events: triggering reload for subsystem %q", subsystem)
		return reloadFn(subsystem)
	}
}

// Action represents an action to take in response to an event
type Action struct {
	Name        string
	Description string
	Execute     func(ctx context.Context, event Event) error
}

// ActionRegistry holds available actions
type ActionRegistry struct {
	actions map[string]Action
	mu      sync.RWMutex
}

// NewActionRegistry creates a new action registry
func NewActionRegistry() *ActionRegistry {
	return &ActionRegistry{
		actions: make(map[string]Action),
	}
}

// Register adds an action to the registry
func (r *ActionRegistry) Register(action Action) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.actions[action.Name] = action
}

// Get retrieves an action by name
func (r *ActionRegistry) Get(name string) (Action, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	action, ok := r.actions[name]
	return action, ok
}

// Execute runs an action by name
func (r *ActionRegistry) Execute(ctx context.Context, name string, event Event) error {
	action, ok := r.Get(name)
	if !ok {
		return fmt.Errorf("action %q not found", name)
	}
	return action.Execute(ctx, event)
}

// DefaultActions returns commonly used actions
func DefaultActions() *ActionRegistry {
	registry := NewActionRegistry()

	registry.Register(Action{
		Name:        "log",
		Description: "Log the event",
		Execute: func(ctx context.Context, event Event) error {
			log.Printf("ACTION[log]: %s", event.String())
			return nil
		},
	})

	registry.Register(Action{
		Name:        "notify",
		Description: "Send notification (placeholder)",
		Execute: func(ctx context.Context, event Event) error {
			log.Printf("ACTION[notify]: Would notify about: %s", event.String())
			// TODO: Implement actual notification (slack, email, etc.)
			return nil
		},
	})

	return registry
}
