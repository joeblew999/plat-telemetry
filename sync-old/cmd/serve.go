package cmd

import (
	"os"
	"strings"
	"time"

	"github.com/joeblew99/plat-telemetry/sync/pkg/server"
)

// Serve runs the sync service (webhook server + optional tunnel)
func Serve() {
	opts := server.Options{
		Port:    getEnv("PORT", "9090"),
		SmeeURL: os.Getenv("SMEE_URL"),
		DataDir: getEnv("SYNC_DATA", ".data"),

		// Tunnel type
		TunnelType: parseTunnelType(os.Getenv("TUNNEL_TYPE")),

		// Cloudflare options
		EnableCF:        parseBool(os.Getenv("SYNC_ENABLE_CF")),
		CFAccountID:     os.Getenv("CF_ACCOUNT_ID"),
		CFAPIToken:      os.Getenv("CF_API_TOKEN"),
		CFWebhookSecret: os.Getenv("CF_WEBHOOK_SECRET"),
		CFAuditPoll:     parseBool(os.Getenv("SYNC_ENABLE_CF_AUDIT")),
		CFPollInterval:  parseDuration(os.Getenv("SYNC_POLL_INTERVAL"), time.Minute),
	}

	// Auto-enable CF if credentials are present
	if opts.CFAccountID != "" && opts.CFAPIToken != "" {
		opts.EnableCF = true
	}

	server.Run(opts)
}

// getEnv returns the environment variable value or a default
func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// parseTunnelType parses TUNNEL_TYPE env var
func parseTunnelType(s string) server.TunnelType {
	switch strings.ToLower(s) {
	case "cloudflared", "cf":
		return server.TunnelCloudflare
	case "smee":
		return server.TunnelSmee
	case "none", "":
		return server.TunnelNone
	default:
		return server.TunnelNone
	}
}

// parseBool parses a boolean env var
func parseBool(s string) bool {
	switch strings.ToLower(s) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// parseDuration parses a duration env var with default
func parseDuration(s string, defaultVal time.Duration) time.Duration {
	if s == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultVal
	}
	return d
}
