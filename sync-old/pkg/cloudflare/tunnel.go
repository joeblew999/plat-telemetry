package cloudflare

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// TunnelConfig holds configuration for cloudflared tunnel
type TunnelConfig struct {
	// Name is the tunnel name (creates quick tunnel if empty)
	Name string

	// LocalPort is the local port to expose (e.g., 8080)
	LocalPort int

	// Hostname is the custom hostname (optional, for named tunnels)
	Hostname string

	// Protocol to use: http or https (default: http)
	Protocol string

	// CloudflaredPath is the path to cloudflared binary (auto-detect if empty)
	CloudflaredPath string
}

// Tunnel manages a cloudflared tunnel
type Tunnel struct {
	config   TunnelConfig
	cmd      *exec.Cmd
	url      string
	urlCh    chan string
	stopCh   chan struct{}
	mu       sync.Mutex
	running  bool
}

// NewTunnel creates a new Cloudflare tunnel manager
func NewTunnel(cfg TunnelConfig) *Tunnel {
	if cfg.Protocol == "" {
		cfg.Protocol = "http"
	}
	if cfg.CloudflaredPath == "" {
		cfg.CloudflaredPath = "cloudflared"
	}

	return &Tunnel{
		config: cfg,
		urlCh:  make(chan string, 1),
		stopCh: make(chan struct{}),
	}
}

// Start starts the cloudflared tunnel
func (t *Tunnel) Start(ctx context.Context) error {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return fmt.Errorf("tunnel already running")
	}
	t.running = true
	t.mu.Unlock()

	// Build command args
	args := []string{"tunnel"}

	if t.config.Name == "" {
		// Quick tunnel (no account needed)
		args = append(args, "--url", fmt.Sprintf("%s://localhost:%d", t.config.Protocol, t.config.LocalPort))
	} else {
		// Named tunnel (requires auth)
		args = append(args, "run", t.config.Name)
	}

	t.cmd = exec.CommandContext(ctx, t.config.CloudflaredPath, args...)

	// Capture stderr for URL extraction (cloudflared outputs URL to stderr)
	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Also capture stdout
	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start cloudflared: %w", err)
	}

	log.Printf("cloudflared: tunnel starting (pid: %d)", t.cmd.Process.Pid)

	// Parse output for tunnel URL
	go t.parseOutput(bufio.NewReader(stderr), "stderr")
	go t.parseOutput(bufio.NewReader(stdout), "stdout")

	// Wait for URL or timeout
	select {
	case url := <-t.urlCh:
		t.url = url
		log.Printf("cloudflared: tunnel ready at %s", url)
	case <-time.After(30 * time.Second):
		t.Stop()
		return fmt.Errorf("timeout waiting for tunnel URL")
	case <-ctx.Done():
		t.Stop()
		return ctx.Err()
	}

	return nil
}

// parseOutput reads cloudflared output and extracts the tunnel URL
func (t *Tunnel) parseOutput(r *bufio.Reader, source string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		// Log cloudflared output
		log.Printf("cloudflared [%s]: %s", source, line)

		// Look for tunnel URL in output
		// Format: "... Your quick Tunnel has been created! Visit it at (it may take some time to be reachable): https://xxx.trycloudflare.com"
		// Or: "... https://xxx.trycloudflare.com"
		if strings.Contains(line, "trycloudflare.com") || strings.Contains(line, ".cfargotunnel.com") {
			url := extractURL(line)
			if url != "" {
				select {
				case t.urlCh <- url:
				default:
				}
			}
		}
	}
}

// extractURL extracts a URL from a log line
func extractURL(line string) string {
	// Find https:// and extract until space or end
	idx := strings.Index(line, "https://")
	if idx == -1 {
		return ""
	}

	url := line[idx:]
	// Trim at first space or newline
	if spaceIdx := strings.IndexAny(url, " \t\n\r"); spaceIdx != -1 {
		url = url[:spaceIdx]
	}

	return url
}

// Stop stops the tunnel
func (t *Tunnel) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return
	}

	if t.cmd != nil && t.cmd.Process != nil {
		log.Printf("cloudflared: stopping tunnel")
		t.cmd.Process.Kill()
		t.cmd.Wait()
	}

	t.running = false
	close(t.stopCh)
}

// URL returns the tunnel's public URL
func (t *Tunnel) URL() string {
	return t.url
}

// IsRunning returns whether the tunnel is running
func (t *Tunnel) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}

// Wait waits for the tunnel to exit
func (t *Tunnel) Wait() error {
	if t.cmd == nil {
		return nil
	}
	return t.cmd.Wait()
}

// RunQuickTunnel starts a quick tunnel and returns the URL
// This is a convenience function for simple use cases
func RunQuickTunnel(ctx context.Context, localPort int) (string, *Tunnel, error) {
	tunnel := NewTunnel(TunnelConfig{
		LocalPort: localPort,
	})

	if err := tunnel.Start(ctx); err != nil {
		return "", nil, err
	}

	return tunnel.URL(), tunnel, nil
}

// CheckCloudflared verifies cloudflared is installed
func CheckCloudflared() error {
	cmd := exec.Command("cloudflared", "version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("cloudflared not found: %w (install from https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/install-and-setup/installation)", err)
	}
	log.Printf("cloudflared: found version %s", strings.TrimSpace(string(output)))
	return nil
}

// InstallCloudflared attempts to install cloudflared
func InstallCloudflared() error {
	// Detect OS and install accordingly
	switch {
	case fileExists("/opt/homebrew/bin/brew") || fileExists("/usr/local/bin/brew"):
		// macOS with Homebrew
		cmd := exec.Command("brew", "install", "cloudflared")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	default:
		return fmt.Errorf("automatic installation not supported for this OS, please install manually from https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/install-and-setup/installation")
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// SetupWebhookEndpoint is a helper to configure webhooks pointing to the tunnel
func (t *Tunnel) SetupWebhookEndpoint(path string) string {
	if t.url == "" {
		return ""
	}
	return t.url + path
}
