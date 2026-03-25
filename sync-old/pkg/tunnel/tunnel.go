package tunnel

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// smeePayload represents the wrapper format smee.io uses
type smeePayload struct {
	Body             json.RawMessage `json:"body"`
	Timestamp        int64           `json:"timestamp"`
	XGitHubEvent     string          `json:"x-github-event"`
	XGitHubDelivery  string          `json:"x-github-delivery"`
	XHubSignature    string          `json:"x-hub-signature"`
	XHubSignature256 string          `json:"x-hub-signature-256"`
}

// Run connects to smee.io and forwards webhooks to local server
// This is a pure Go implementation of smee-client with auto-reconnect
func Run(smeeURL, target string) {
	for {
		log.Printf("▶ Connecting to %s", smeeURL)
		log.Printf("▶ Forwarding to %s", target)

		err := connectAndForward(smeeURL, target)
		if err != nil {
			log.Printf("⚠️  Connection error: %v", err)
			log.Printf("⏳ Reconnecting in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		}
	}
}

// GenerateChannel creates a new smee.io channel URL
func GenerateChannel() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("❌ Failed to generate channel ID: %v", err)
	}

	channelID := base64.RawURLEncoding.EncodeToString(b)
	return fmt.Sprintf("https://smee.io/%s", channelID)
}

// ConfigureGitHubWebhook uses gh CLI to create a webhook for a repo
func ConfigureGitHubWebhook(repo, webhookURL, events string) error {
	// Check if webhook already exists
	checkCmd := exec.Command("gh", "api", fmt.Sprintf("repos/%s/hooks", repo), "--jq", ".[].config.url")
	output, _ := checkCmd.Output()

	existingWebhooks := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, url := range existingWebhooks {
		if strings.Contains(url, "smee.io") {
			log.Printf("⚠️  Existing smee webhook found, will create new one...")
		}
	}

	// Build the JSON payload directly
	eventList := strings.Split(events, ",")
	payload := map[string]any{
		"name":   "web",
		"active": true,
		"events": eventList,
		"config": map[string]string{
			"url":          webhookURL,
			"content_type": "json",
			"insecure_ssl": "0",
		},
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create webhook using gh api with JSON input
	createCmd := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/hooks", repo),
		"-X", "POST",
		"--input", "-",
	)
	createCmd.Stdin = bytes.NewReader(payloadJSON)

	if output, err := createCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gh api failed: %w\n%s", err, string(output))
	}

	return nil
}

func connectAndForward(smeeURL, target string) error {
	req, err := http.NewRequest("GET", smeeURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{
		Timeout: 0, // No timeout for SSE
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("smee returned status %d", resp.StatusCode)
	}

	log.Printf("✅ Connected to smee.io")

	// Read SSE events
	reader := bufio.NewReader(resp.Body)
	var eventData strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return fmt.Errorf("connection closed")
			}
			return fmt.Errorf("read error: %w", err)
		}

		line = strings.TrimSpace(line)

		// Empty line = end of event
		if line == "" {
			if eventData.Len() > 0 {
				forwardEvent(target, eventData.String())
				eventData.Reset()
			}
			continue
		}

		// Parse SSE format
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)
			eventData.WriteString(data)
		}
		// Ignore other SSE fields (event:, id:, retry:)
	}
}

// forwardEvent sends the webhook payload to the local target
func forwardEvent(target, data string) {
	// Skip ready/ping events
	if data == "ready" || data == "" {
		return
	}

	// Parse smee wrapper to extract headers and body
	var payload smeePayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		// Not JSON or not smee format - forward as-is
		log.Printf("📨 Forwarding raw event...")
		forwardRaw(target, data, nil)
		return
	}

	// Get the body - either from wrapper or use whole payload
	var body []byte
	if len(payload.Body) > 0 {
		body = payload.Body
	} else {
		body = []byte(data)
	}

	// Build headers map from parsed fields
	headers := make(map[string]string)
	if payload.XGitHubEvent != "" {
		headers["X-GitHub-Event"] = payload.XGitHubEvent
	}
	if payload.XGitHubDelivery != "" {
		headers["X-GitHub-Delivery"] = payload.XGitHubDelivery
	}
	if payload.XHubSignature != "" {
		headers["X-Hub-Signature"] = payload.XHubSignature
	}
	if payload.XHubSignature256 != "" {
		headers["X-Hub-Signature-256"] = payload.XHubSignature256
	}

	// Extract event type for logging
	eventType := payload.XGitHubEvent
	if eventType == "" {
		eventType = "unknown"
	}

	log.Printf("📨 Forwarding %s event...", eventType)
	forwardRaw(target, string(body), headers)
}

func forwardRaw(target, body string, headers map[string]string) {
	req, err := http.NewRequest("POST", target, bytes.NewBufferString(body))
	if err != nil {
		log.Printf("❌ Failed to create forward request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ Failed to forward: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("✅ Forwarded successfully (status %d)", resp.StatusCode)
	} else {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("⚠️  Forward returned %d: %s", resp.StatusCode, string(respBody))
	}
}
