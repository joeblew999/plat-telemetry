package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// AlertPayload represents a Cloudflare notification webhook payload
type AlertPayload struct {
	Name        string                 `json:"name"`
	Text        string                 `json:"text"`
	Data        map[string]interface{} `json:"data"`
	Timestamp   string                 `json:"ts"`
	AccountID   string                 `json:"account_id"`
	AccountName string                 `json:"account_name"`
	AlertType   string                 `json:"alert_type"`
}

// WebhookHandler handles incoming Cloudflare webhook events
type WebhookHandler struct {
	client     *Client
	secretKey  string // Optional: for webhook signature verification
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(client *Client, secretKey string) *WebhookHandler {
	return &WebhookHandler{
		client:    client,
		secretKey: secretKey,
	}
}

// ServeHTTP handles incoming webhook requests
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("cloudflare webhook: failed to read body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// TODO: Verify webhook signature if secretKey is set
	// CF uses webhook signatures for some services

	// Try to parse as alert payload
	var alert AlertPayload
	if err := json.Unmarshal(body, &alert); err != nil {
		log.Printf("cloudflare webhook: failed to parse payload: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Parse timestamp
	ts, err := time.Parse(time.RFC3339, alert.Timestamp)
	if err != nil {
		ts = time.Now()
	}

	// Determine event type from alert_type
	eventType := EventAlert
	resource := alert.AlertType

	// Map specific alert types to more specific event types
	switch alert.AlertType {
	case "pages_event":
		eventType = EventPagesDeploy
		resource = "pages"
	case "workers_event":
		eventType = EventWorkersDeploy
		resource = "workers"
	case "tunnel_health_event":
		eventType = EventTunnel
		resource = "tunnel"
	}

	event := Event{
		Type:      eventType,
		Timestamp: ts,
		AccountID: alert.AccountID,
		Action:    alert.Name,
		Resource:  resource,
		Metadata: map[string]interface{}{
			"text":         alert.Text,
			"alert_type":   alert.AlertType,
			"account_name": alert.AccountName,
			"data":         alert.Data,
		},
		Raw: alert,
	}

	log.Printf("cloudflare webhook: received %s event: %s", event.Type, event.Action)
	h.client.emit(r.Context(), event)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

// HandleWebhook returns an http.HandlerFunc for use with standard routers
func (c *Client) HandleWebhook(secretKey string) http.HandlerFunc {
	handler := NewWebhookHandler(c, secretKey)
	return handler.ServeHTTP
}

// LogpushPayload represents a Cloudflare Logpush webhook payload
// Logpush can send to HTTP endpoints with custom headers
type LogpushPayload struct {
	// Logpush sends newline-delimited JSON (NDJSON)
	// Each line is a log entry, structure varies by dataset
	Entries []map[string]interface{}
}

// LogpushHandler handles incoming Logpush webhook events
type LogpushHandler struct {
	client  *Client
	dataset string // e.g., "http_requests", "firewall_events"
}

// NewLogpushHandler creates a new Logpush handler
func NewLogpushHandler(client *Client, dataset string) *LogpushHandler {
	return &LogpushHandler{
		client:  client,
		dataset: dataset,
	}
}

// ServeHTTP handles incoming Logpush requests
func (h *LogpushHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("cloudflare logpush: failed to read body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Logpush sends gzip-compressed NDJSON by default
	// For now, assume uncompressed JSON array
	var entries []map[string]interface{}
	if err := json.Unmarshal(body, &entries); err != nil {
		log.Printf("cloudflare logpush: failed to parse payload: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	log.Printf("cloudflare logpush: received %d entries for dataset %s", len(entries), h.dataset)

	// Emit a single event with all entries (batch processing)
	event := Event{
		Type:      EventLogpush,
		Timestamp: time.Now(),
		AccountID: h.client.accountID,
		Action:    "logpush_batch",
		Resource:  h.dataset,
		Metadata: map[string]interface{}{
			"count":   len(entries),
			"dataset": h.dataset,
		},
		Raw: entries,
	}

	h.client.emit(r.Context(), event)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

// HandleLogpush returns an http.HandlerFunc for Logpush webhooks
func (c *Client) HandleLogpush(dataset string) http.HandlerFunc {
	handler := NewLogpushHandler(c, dataset)
	return handler.ServeHTTP
}

// RegisterRoutes registers all Cloudflare webhook routes on a mux
func (c *Client) RegisterRoutes(mux *http.ServeMux, basePath string, secretKey string) {
	// Main alert webhook
	mux.HandleFunc(basePath+"/webhook", c.HandleWebhook(secretKey))
	mux.HandleFunc(basePath+"/webhook/", c.HandleWebhook(secretKey))

	// Logpush endpoints by dataset
	mux.HandleFunc(basePath+"/logpush/http_requests", c.HandleLogpush("http_requests"))
	mux.HandleFunc(basePath+"/logpush/firewall_events", c.HandleLogpush("firewall_events"))
	mux.HandleFunc(basePath+"/logpush/audit_logs", c.HandleLogpush("audit_logs"))
}

// SetupWebhookDestination creates a webhook destination in Cloudflare
// This requires the alerting:write scope
func (c *Client) SetupWebhookDestination(ctx context.Context, name, webhookURL, secret string) error {
	// This will use cloudflare-go SDK in the future
	// For now, just log the intent
	log.Printf("cloudflare: would create webhook destination: name=%s url=%s", name, webhookURL)

	// TODO: Implement with cloudflare-go/v6
	// client.Alerting.Destinations.Webhooks.New(ctx, alerting.DestinationWebhookNewParams{
	//     AccountID: cloudflare.F(c.accountID),
	//     Name:      cloudflare.F(name),
	//     URL:       cloudflare.F(webhookURL),
	//     Secret:    cloudflare.F(secret),
	// })

	return nil
}
