package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joeblew99/plat-telemetry/sync/pkg/cloudflare"
)

// CFTunnel starts a cloudflared quick tunnel
func CFTunnel(args []string) {
	port := 9090
	if len(args) > 0 {
		fmt.Sscanf(args[0], "%d", &port)
	}

	// Check cloudflared is installed
	if err := cloudflare.CheckCloudflared(); err != nil {
		log.Printf("⚠️  cloudflared not found, attempting install...")
		if err := cloudflare.InstallCloudflared(); err != nil {
			log.Fatalf("❌ cloudflared not available: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT/SIGTERM
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		cancel()
	}()

	t := cloudflare.NewTunnel(cloudflare.TunnelConfig{
		LocalPort: port,
	})

	log.Printf("▶ Starting cloudflared quick tunnel for localhost:%d...", port)

	if err := t.Start(ctx); err != nil {
		log.Fatalf("❌ Failed to start tunnel: %v", err)
	}

	log.Printf("🌐 Tunnel URL: %s", t.URL())
	log.Printf("   Webhook endpoint: %s/webhook", t.URL())
	log.Printf("   CF webhook endpoint: %s/cf/webhook", t.URL())
	log.Printf("")
	log.Printf("Press Ctrl+C to stop the tunnel")

	// Wait for context cancellation
	<-ctx.Done()
	t.Stop()
	log.Printf("⏹️  Tunnel stopped")
}

// CFPoll polls Cloudflare audit logs
func CFPoll(args []string) {
	accountID := os.Getenv("CF_ACCOUNT_ID")
	apiToken := os.Getenv("CF_API_TOKEN")

	if accountID == "" || apiToken == "" {
		log.Fatal("❌ CF_ACCOUNT_ID and CF_API_TOKEN environment variables required")
	}

	interval := time.Minute
	if len(args) > 0 {
		if d, err := time.ParseDuration(args[0]); err == nil {
			interval = d
		}
	}

	client, err := cloudflare.NewClient(cloudflare.Config{
		APIToken:     apiToken,
		AccountID:    accountID,
		PollInterval: interval,
	})
	if err != nil {
		log.Fatalf("❌ Failed to create CF client: %v", err)
	}

	// Log all events
	client.OnAny(func(ctx context.Context, event cloudflare.Event) error {
		log.Printf("EVENT: [%s] %s on %s by %s",
			event.Type, event.Action, event.Resource, event.Actor)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT/SIGTERM
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		cancel()
	}()

	log.Printf("▶ Starting CF audit log polling (interval: %s)", interval)
	log.Printf("  Account: %s", accountID)
	log.Printf("")
	log.Printf("Press Ctrl+C to stop")

	poller := cloudflare.NewAuditPoller(client, interval)
	poller.Start(ctx)
}

// CFWebhook starts a CF webhook server only (no GitHub)
func CFWebhook(args []string) {
	port := "9090"
	if len(args) > 0 {
		port = args[0]
	}

	accountID := os.Getenv("CF_ACCOUNT_ID")
	apiToken := os.Getenv("CF_API_TOKEN")
	webhookSecret := os.Getenv("CF_WEBHOOK_SECRET")

	if accountID == "" {
		log.Printf("⚠️  CF_ACCOUNT_ID not set - some features may not work")
	}

	// Create a minimal client for webhook handling
	var client *cloudflare.Client
	if accountID != "" && apiToken != "" {
		var err error
		client, err = cloudflare.NewClient(cloudflare.Config{
			APIToken:  apiToken,
			AccountID: accountID,
		})
		if err != nil {
			log.Fatalf("❌ Failed to create CF client: %v", err)
		}

		// Log all events
		client.OnAny(func(ctx context.Context, event cloudflare.Event) error {
			log.Printf("EVENT: [%s] %s on %s", event.Type, event.Action, event.Resource)
			if event.Actor != "" {
				log.Printf("  Actor: %s", event.Actor)
			}
			return nil
		})
	}

	// Start HTTP server
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	if client != nil {
		client.RegisterRoutes(mux, "/cf", webhookSecret)
	}

	log.Printf("▶ CF webhook server listening on :%s", port)
	log.Printf("   Health: http://localhost:%s/health", port)
	log.Printf("   Webhook: http://localhost:%s/cf/webhook", port)
	log.Printf("   Logpush: http://localhost:%s/cf/logpush/*", port)

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

// CFCheck checks cloudflared installation
func CFCheck() {
	if err := cloudflare.CheckCloudflared(); err != nil {
		log.Printf("❌ cloudflared not installed")
		log.Printf("   Run: sync cf install")
		os.Exit(1)
	}
	log.Printf("✅ cloudflared is installed")
}

// CFInstall installs cloudflared
func CFInstall() {
	if err := cloudflare.InstallCloudflared(); err != nil {
		log.Fatalf("❌ Failed to install cloudflared: %v", err)
	}
	log.Printf("✅ cloudflared installed successfully")
}
