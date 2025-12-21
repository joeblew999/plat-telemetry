package cmd

import (
	"log"

	taskfilepoller "github.com/joeblew99/plat-telemetry/sync/pkg/taskfile-poller"
)

// PollTaskfiles starts the Taskfile polling loop
func PollTaskfiles() {
	log.Println("🔄 sync poll-taskfiles - Monitor Taskfiles for version changes")

	p := taskfilepoller.NewTaskfilePoller()
	if err := p.Start(); err != nil {
		log.Fatalf("❌ Taskfile poller failed: %v", err)
	}
}
