package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/joeblew99/plat-telemetry/sync/pkg/cloudflare"
	cftunnel "github.com/joeblew99/plat-telemetry/sync/pkg/cloudflare"
	"github.com/joeblew99/plat-telemetry/sync/pkg/events"
	"github.com/joeblew99/plat-telemetry/sync/pkg/tunnel"
	"github.com/joeblew99/plat-telemetry/sync/pkg/webhook"
)

// TunnelType specifies which tunnel to use
type TunnelType string

const (
	TunnelNone       TunnelType = "none"
	TunnelSmee       TunnelType = "smee"
	TunnelCloudflare TunnelType = "cloudflared"
)

// Config holds the sync service configuration
type Config struct {
	SmeeURL    string `json:"smee_url"`
	Repo       string `json:"repo"`
	TunnelType string `json:"tunnel_type,omitempty"`

	// Cloudflare config
	CFAccountID     string `json:"cf_account_id,omitempty"`
	CFAPIToken      string `json:"cf_api_token,omitempty"`
	CFWebhookSecret string `json:"cf_webhook_secret,omitempty"`
	CFAuditPoll     bool   `json:"cf_audit_poll,omitempty"`
	CFPollInterval  string `json:"cf_poll_interval,omitempty"`
}

// Options configures the server
type Options struct {
	Port    string
	SmeeURL string
	DataDir string

	// Tunnel options
	TunnelType TunnelType

	// Cloudflare options
	EnableCF        bool
	CFAccountID     string
	CFAPIToken      string
	CFWebhookSecret string
	CFAuditPoll     bool
	CFPollInterval  time.Duration
}

// Server is the main sync server
type Server struct {
	opts     Options
	eventBus *events.Bus
	cfClient *cloudflare.Client
	mux      *http.ServeMux
	cfTunnel *cftunnel.Tunnel

	// Shutdown coordination
	shutdownCh chan struct{}
}

// NewServer creates a new sync server
func NewServer(opts Options) *Server {
	if opts.Port == "" {
		opts.Port = "9090"
	}
	if opts.DataDir == "" {
		opts.DataDir = ".data"
	}
	if opts.CFPollInterval == 0 {
		opts.CFPollInterval = time.Minute
	}

	return &Server{
		opts:       opts,
		eventBus:   events.NewBus(),
		mux:        http.NewServeMux(),
		shutdownCh: make(chan struct{}),
	}
}

// Run starts the sync service
func (s *Server) Run(ctx context.Context) error {
	// Load config if exists
	config := loadConfig(s.opts.DataDir)
	s.applyConfig(config)

	// Setup event handlers
	s.setupEventHandlers()

	// Setup HTTP routes
	s.setupRoutes()

	// Setup Cloudflare if enabled
	if s.opts.EnableCF {
		if err := s.setupCloudflare(ctx); err != nil {
			log.Printf("⚠️  Cloudflare setup failed: %v", err)
			// Continue without CF - not fatal
		}
	}

	// Start tunnel based on type
	if err := s.startTunnel(ctx); err != nil {
		log.Printf("⚠️  Tunnel start failed: %v", err)
		// Continue without tunnel - not fatal
	}

	// Handle shutdown gracefully
	go s.handleShutdown()

	// Start server
	addr := fmt.Sprintf(":%s", s.opts.Port)
	log.Printf("▶ Sync service listening on %s", addr)

	server := &http.Server{
		Addr:    addr,
		Handler: s.mux,
	}

	// Shutdown on context cancel
	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// applyConfig applies loaded config to options
func (s *Server) applyConfig(config Config) {
	if s.opts.SmeeURL == "" && config.SmeeURL != "" {
		s.opts.SmeeURL = config.SmeeURL
		log.Printf("📋 Using saved smee URL: %s", s.opts.SmeeURL)
	}

	// Apply tunnel type from config if not set
	if s.opts.TunnelType == "" {
		switch config.TunnelType {
		case "cloudflared":
			s.opts.TunnelType = TunnelCloudflare
		case "smee":
			s.opts.TunnelType = TunnelSmee
		default:
			// Auto-detect based on available config
			if s.opts.SmeeURL != "" {
				s.opts.TunnelType = TunnelSmee
			}
		}
	}

	// Apply CF config
	if s.opts.CFAccountID == "" && config.CFAccountID != "" {
		s.opts.CFAccountID = config.CFAccountID
	}
	if s.opts.CFAPIToken == "" && config.CFAPIToken != "" {
		s.opts.CFAPIToken = config.CFAPIToken
	}
	if s.opts.CFWebhookSecret == "" && config.CFWebhookSecret != "" {
		s.opts.CFWebhookSecret = config.CFWebhookSecret
	}

	// Enable CF if we have credentials
	if s.opts.CFAccountID != "" && s.opts.CFAPIToken != "" {
		s.opts.EnableCF = true
	}
}

// setupEventHandlers registers default event handlers
func (s *Server) setupEventHandlers() {
	// Log all events
	s.eventBus.On("logger", events.LogHandler())

	// Add subsystem mapper for reload actions
	mapper := &events.SubsystemMapper{
		RepoToSubsystem: map[string]string{
			"joeblew999/plat-telemetry": "sync",
			// Add more repo mappings as needed
		},
		CFResourceToSubsystem: map[string]string{
			"pages": "docs",
			// Add more CF resource mappings as needed
		},
	}

	// Reload handler
	s.eventBus.OnFiltered("reloader", nil, events.ReloadHandler(mapper, func(subsystem string) error {
		log.Printf("🔄 Would reload subsystem: %s", subsystem)
		// TODO: Call task reload PROC=<subsystem>
		return nil
	}))
}

// setupRoutes configures HTTP endpoints
func (s *Server) setupRoutes() {
	// Health check
	s.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	// Status endpoint
	s.mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]any{
			"service":     "sync",
			"port":        s.opts.Port,
			"smee_url":    s.opts.SmeeURL,
			"tunnel_type": s.opts.TunnelType,
			"cf_enabled":  s.opts.EnableCF,
		}
		if s.cfTunnel != nil && s.cfTunnel.URL() != "" {
			status["tunnel_url"] = s.cfTunnel.URL()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	// GitHub webhook endpoint
	webhookServer := webhook.NewServer()
	s.mux.HandleFunc("/webhook", s.wrapWithEvents(webhookServer.HandleWebhook))
	s.mux.HandleFunc("/webhook/", s.wrapWithEvents(webhookServer.HandleWebhook))
	s.mux.HandleFunc("/webhook/github", s.wrapWithEvents(webhookServer.HandleWebhook))
	s.mux.HandleFunc("/webhook/github/", s.wrapWithEvents(webhookServer.HandleWebhook))
}

// wrapWithEvents wraps a handler to emit events to the bus
func (s *Server) wrapWithEvents(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Call original handler
		handler(w, r)

		// Emit event to bus (GitHub events are handled in webhook package)
		// The webhook package will need to be updated to use event bus
	}
}

// setupCloudflare initializes CF client and routes
func (s *Server) setupCloudflare(ctx context.Context) error {
	if s.opts.CFAccountID == "" || s.opts.CFAPIToken == "" {
		return fmt.Errorf("CF_ACCOUNT_ID and CF_API_TOKEN required")
	}

	client, err := cloudflare.NewClient(cloudflare.Config{
		APIToken:     s.opts.CFAPIToken,
		AccountID:    s.opts.CFAccountID,
		PollInterval: s.opts.CFPollInterval,
	})
	if err != nil {
		return fmt.Errorf("failed to create CF client: %w", err)
	}
	s.cfClient = client

	// Bridge CF events to unified event bus
	client.OnAny(func(ctx context.Context, cfEvent cloudflare.Event) error {
		// Convert CF event to unified event
		event := events.Event{
			ID:        fmt.Sprintf("cf-%d", time.Now().UnixNano()),
			Source:    events.SourceCloudflare,
			Type:      events.Type(cfEvent.Type),
			Timestamp: cfEvent.Timestamp,
			AccountID: cfEvent.AccountID,
			ZoneID:    cfEvent.ZoneID,
			Action:    cfEvent.Action,
			Resource:  cfEvent.Resource,
			Actor:     cfEvent.Actor,
			Metadata:  cfEvent.Metadata,
			Raw:       cfEvent.Raw,
		}
		s.eventBus.Emit(ctx, event)
		return nil
	})

	// Register CF webhook routes
	client.RegisterRoutes(s.mux, "/cf", s.opts.CFWebhookSecret)

	log.Printf("✅ Cloudflare integration enabled (account: %s)", s.opts.CFAccountID)

	// Start audit log polling if enabled
	if s.opts.CFAuditPoll {
		poller := cloudflare.NewAuditPoller(client, s.opts.CFPollInterval)
		go poller.Start(ctx)
		log.Printf("📊 CF audit log polling enabled (interval: %s)", s.opts.CFPollInterval)
	}

	return nil
}

// startTunnel starts the appropriate tunnel
func (s *Server) startTunnel(ctx context.Context) error {
	switch s.opts.TunnelType {
	case TunnelCloudflare:
		return s.startCloudflaredTunnel(ctx)
	case TunnelSmee:
		return s.startSmeeTunnel()
	case TunnelNone:
		log.Printf("ℹ️  No tunnel configured")
		return nil
	default:
		// Auto-detect
		if s.opts.SmeeURL != "" {
			return s.startSmeeTunnel()
		}
		log.Printf("ℹ️  No tunnel configured - webhook server only")
		log.Printf("   Set TUNNEL_TYPE=cloudflared or SMEE_URL for tunneling")
		return nil
	}
}

// startSmeeTunnel starts smee.io tunnel
func (s *Server) startSmeeTunnel() error {
	if s.opts.SmeeURL == "" {
		return fmt.Errorf("SMEE_URL required for smee tunnel")
	}

	go func() {
		target := fmt.Sprintf("http://localhost:%s/webhook", s.opts.Port)
		log.Printf("▶ Starting smee tunnel to %s", s.opts.SmeeURL)
		tunnel.Run(s.opts.SmeeURL, target)
	}()

	return nil
}

// startCloudflaredTunnel starts cloudflared quick tunnel
func (s *Server) startCloudflaredTunnel(ctx context.Context) error {
	// Check if cloudflared is installed
	if err := cftunnel.CheckCloudflared(); err != nil {
		log.Printf("⚠️  cloudflared not found, attempting install...")
		if err := cftunnel.InstallCloudflared(); err != nil {
			return fmt.Errorf("cloudflared not available: %w", err)
		}
	}

	// Start quick tunnel
	port := 9090
	fmt.Sscanf(s.opts.Port, "%d", &port)

	t := cftunnel.NewTunnel(cftunnel.TunnelConfig{
		LocalPort: port,
	})
	s.cfTunnel = t

	log.Printf("▶ Starting cloudflared quick tunnel...")
	if err := t.Start(ctx); err != nil {
		return fmt.Errorf("cloudflared tunnel failed: %w", err)
	}

	log.Printf("🌐 Tunnel URL: %s", t.URL())
	log.Printf("   Webhook endpoint: %s/webhook", t.URL())
	log.Printf("   CF webhook endpoint: %s/cf/webhook", t.URL())

	return nil
}

// handleShutdown handles graceful shutdown
func (s *Server) handleShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Printf("⏹️  Shutting down...")

	// Stop cloudflared tunnel if running
	if s.cfTunnel != nil {
		s.cfTunnel.Stop()
	}

	close(s.shutdownCh)
	os.Exit(0)
}

// Run starts the sync service (webhook server + optional tunnel)
// This is the legacy function for backward compatibility
func Run(opts Options) {
	server := NewServer(opts)
	if err := server.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

// loadConfig loads the sync config from data directory
func loadConfig(dataDir string) Config {
	configPath := filepath.Join(dataDir, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}
	}
	return config
}

// SaveConfig saves the sync config to data directory
func SaveConfig(dataDir string, config Config) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(dataDir, "config.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
