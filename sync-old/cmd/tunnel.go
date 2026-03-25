package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joeblew99/plat-telemetry/sync/pkg/tunnel"
)

// Tunnel connects to smee.io and forwards webhooks to local server
func Tunnel(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sync tunnel <smee-url|new> [target]")
		fmt.Println("  smee-url: Your smee.io channel URL (e.g., https://smee.io/abc123)")
		fmt.Println("  new:      Auto-generate a new smee channel")
		fmt.Println("  target:   Local target URL (default: http://localhost:9090/webhook)")
		os.Exit(1)
	}

	smeeURL := args[0]
	target := "http://localhost:9090/webhook"
	if len(args) > 1 {
		target = args[1]
	}

	if smeeURL == "new" {
		smeeURL = tunnel.GenerateChannel()
		log.Printf("✅ Created channel: %s", smeeURL)
	}

	tunnel.Run(smeeURL, target)
}

// TunnelSetup creates a smee channel and configures GitHub webhook for a repo
func TunnelSetup(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sync tunnel-setup <owner/repo> [--events=push,release,workflow_run]")
		fmt.Println("  Creates a smee channel and configures GitHub webhook")
		os.Exit(1)
	}

	repo := args[0]
	events := "push,release,workflow_run,page_build,deployment_status"

	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "--events=") {
			events = strings.TrimPrefix(arg, "--events=")
		}
	}

	smeeURL := tunnel.GenerateChannel()
	log.Printf("✅ Created smee channel: %s", smeeURL)

	if err := tunnel.ConfigureGitHubWebhook(repo, smeeURL, events); err != nil {
		log.Fatalf("❌ Failed to configure webhook: %v", err)
	}

	log.Printf("✅ Webhook configured for %s", repo)
	log.Printf("")
	log.Printf("To start receiving webhooks:")
	log.Printf("  sync tunnel %s", smeeURL)
}
