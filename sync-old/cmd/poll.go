package cmd

import (
	"log"

	"github.com/joeblew99/plat-telemetry/sync/pkg/poller"
)

// Poll starts the polling loop for upstream repositories
func Poll() {
	log.Println("🔄 sync poll - Monitor upstream repositories for updates")

	p := poller.NewPoller()
	if err := p.Start(); err != nil {
		log.Fatalf("❌ Poller failed: %v", err)
	}
}
