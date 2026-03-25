// Package main implements a Cloudflare Worker for event aggregation.
// This worker receives events from various Cloudflare sources and forwards
// them to your sync service endpoint.
//
// Deploy: task cf-worker:deploy
// Dev: task cf-worker:run
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/syumai/workers"
	"github.com/syumai/workers/cloudflare/fetch"
)

// Event represents a normalized Cloudflare event
type Event struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	AccountID string                 `json:"account_id,omitempty"`
	ZoneID    string                 `json:"zone_id,omitempty"`
	Action    string                 `json:"action"`
	Resource  string                 `json:"resource"`
	Source    string                 `json:"source"` // Which CF service sent this
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Raw       json.RawMessage        `json:"raw,omitempty"`
}

// Usage tracks request counts for billing visibility
type Usage struct {
	TotalRequests   int64 `json:"total_requests"`
	WebhookPages    int64 `json:"webhook_pages"`
	WebhookAlert    int64 `json:"webhook_alert"`
	Logpush         int64 `json:"logpush"`
	ForwardSuccess  int64 `json:"forward_success"`
	ForwardFailures int64 `json:"forward_failures"`
}

// Version set by ldflags at build time
var version = "dev"

// In-memory usage counters (reset on worker restart)
var usage Usage

// Config from environment variables
var (
	syncEndpoint string // Where to forward events (e.g., your tunnel URL)
	syncToken    string // Auth token for your sync service
	workerName   string // Worker name for identification
)

func init() {
	syncEndpoint = os.Getenv("SYNC_ENDPOINT")
	syncToken = os.Getenv("SYNC_TOKEN")
	workerName = os.Getenv("WORKER_NAME")
	if workerName == "" {
		workerName = "plat-event-aggregator"
	}
}

func main() {
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/metrics", handleMetrics)
	http.HandleFunc("/webhook/pages", handlePagesWebhook)
	http.HandleFunc("/webhook/alert", handleAlertWebhook)
	http.HandleFunc("/logpush", handleLogpush)

	workers.Serve(nil)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service": workerName,
		"version": version,
		"endpoints": []string{
			"/health",
			"/metrics",
			"/webhook/pages",
			"/webhook/alert",
			"/logpush",
		},
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

// handleMetrics returns usage metrics for billing visibility
func handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Snapshot current usage
	metrics := map[string]interface{}{
		"worker":  workerName,
		"version": version,
		"usage": map[string]int64{
			"total_requests":   atomic.LoadInt64(&usage.TotalRequests),
			"webhook_pages":    atomic.LoadInt64(&usage.WebhookPages),
			"webhook_alert":    atomic.LoadInt64(&usage.WebhookAlert),
			"logpush":          atomic.LoadInt64(&usage.Logpush),
			"forward_success":  atomic.LoadInt64(&usage.ForwardSuccess),
			"forward_failures": atomic.LoadInt64(&usage.ForwardFailures),
		},
		"config": map[string]interface{}{
			"sync_endpoint_configured": syncEndpoint != "",
		},
		"billing_note": "Cloudflare Workers: Free tier 100k req/day, Paid $5/mo + $0.50/million after 10M. WASM size affects limits (free: 1MB, paid: 10MB).",
	}

	json.NewEncoder(w).Encode(metrics)
}

// handlePagesWebhook handles Cloudflare Pages deploy hooks
func handlePagesWebhook(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&usage.TotalRequests, 1)
	atomic.AddInt64(&usage.WebhookPages, 1)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	event := Event{
		Type:      "pages_deploy",
		Timestamp: time.Now(),
		Action:    "deploy",
		Resource:  "pages",
		Source:    "pages_deploy_hook",
		Metadata:  payload,
		Raw:       body,
	}

	if err := forwardEvent(r.Context(), event); err != nil {
		log.Printf("forward error: %v", err)
		// Still return OK to CF - we don't want retries flooding us
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

// handleAlertWebhook handles Cloudflare Notifications webhooks
func handleAlertWebhook(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&usage.TotalRequests, 1)
	atomic.AddInt64(&usage.WebhookAlert, 1)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload struct {
		Name        string                 `json:"name"`
		Text        string                 `json:"text"`
		Data        map[string]interface{} `json:"data"`
		Timestamp   string                 `json:"ts"`
		AccountID   string                 `json:"account_id"`
		AccountName string                 `json:"account_name"`
		AlertType   string                 `json:"alert_type"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	event := Event{
		Type:      mapAlertType(payload.AlertType),
		Timestamp: time.Now(),
		AccountID: payload.AccountID,
		Action:    payload.Name,
		Resource:  payload.AlertType,
		Source:    "notification_webhook",
		Metadata: map[string]interface{}{
			"text":         payload.Text,
			"account_name": payload.AccountName,
			"data":         payload.Data,
		},
		Raw: body,
	}

	if err := forwardEvent(r.Context(), event); err != nil {
		log.Printf("forward error: %v", err)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

// handleLogpush handles Cloudflare Logpush HTTP destination
func handleLogpush(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&usage.TotalRequests, 1)
	atomic.AddInt64(&usage.Logpush, 1)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dataset := r.URL.Query().Get("dataset")
	if dataset == "" {
		dataset = "unknown"
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Logpush sends NDJSON or JSON array
	var entries []map[string]interface{}
	if err := json.Unmarshal(body, &entries); err != nil {
		// Maybe it's NDJSON, log and continue
		log.Printf("logpush: parse error (may be NDJSON): %v", err)
	}

	event := Event{
		Type:      "logpush",
		Timestamp: time.Now(),
		Action:    "batch",
		Resource:  dataset,
		Source:    "logpush",
		Metadata: map[string]interface{}{
			"dataset": dataset,
			"count":   len(entries),
		},
		Raw: body,
	}

	if err := forwardEvent(r.Context(), event); err != nil {
		log.Printf("forward error: %v", err)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

// mapAlertType maps Cloudflare alert types to normalized event types
func mapAlertType(alertType string) string {
	switch alertType {
	case "pages_event":
		return "pages_deploy"
	case "workers_event":
		return "workers_deploy"
	case "tunnel_health_event":
		return "tunnel"
	default:
		return "alert"
	}
}

// forwardEvent sends the event to the sync service
func forwardEvent(ctx context.Context, event Event) error {
	if syncEndpoint == "" {
		log.Printf("SYNC_ENDPOINT not configured, event: %s/%s", event.Type, event.Action)
		return nil
	}

	body, err := json.Marshal(event)
	if err != nil {
		atomic.AddInt64(&usage.ForwardFailures, 1)
		return fmt.Errorf("marshal event: %w", err)
	}

	cli := fetch.NewClient()
	req, err := fetch.NewRequest(ctx, http.MethodPost, syncEndpoint, bytes.NewReader(body))
	if err != nil {
		atomic.AddInt64(&usage.ForwardFailures, 1)
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if syncToken != "" {
		req.Header.Set("Authorization", "Bearer "+syncToken)
	}

	resp, err := cli.Do(req, nil)
	if err != nil {
		atomic.AddInt64(&usage.ForwardFailures, 1)
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		atomic.AddInt64(&usage.ForwardFailures, 1)
		return fmt.Errorf("sync service returned %d", resp.StatusCode)
	}

	atomic.AddInt64(&usage.ForwardSuccess, 1)
	log.Printf("forwarded event: %s/%s", event.Type, event.Action)
	return nil
}
