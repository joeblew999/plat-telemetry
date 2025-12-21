package cmd

import (
	"fmt"

	"github.com/joeblew99/plat-telemetry/sync/pkg/checker"
)

// Check runs version check for all subsystems
func Check() {
	fmt.Println("Checking for upstream updates...")

	subsystems := []string{"arc", "liftbridge", "nats", "telegraf"}

	for _, subsystem := range subsystems {
		current, latest, err := checker.CheckVersion(subsystem)
		if err != nil {
			fmt.Printf("❌ %s: %v\n", subsystem, err)
			continue
		}

		if current == latest {
			fmt.Printf("✅ %s: up-to-date (%s)\n", subsystem, current)
		} else {
			fmt.Printf("🔄 %s: %s → %s (update available)\n", subsystem, current, latest)
		}
	}
}
