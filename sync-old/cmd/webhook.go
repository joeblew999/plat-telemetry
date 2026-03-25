package cmd

import (
	"os"

	"github.com/joeblew99/plat-telemetry/sync/pkg/webhook"
)

// Webhook starts the standalone webhook server
func Webhook() {
	webhook.Run(os.Getenv("PORT"))
}
