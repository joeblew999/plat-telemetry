package webhook

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/exec"

	"github.com/cbrgm/githubevents/v2/githubevents"
	"github.com/google/go-github/v80/github"
)

// Server handles webhook events
type Server struct {
	handler *githubevents.EventHandler
}

// NewServer creates a new webhook server with githubevents
func NewServer() *Server {
	handler := githubevents.New("")

	// Register release event handler
	handler.OnReleaseEventPublished(func(ctx context.Context, deliveryID string, eventName string, event *github.ReleaseEvent) error {
		repo := event.GetRepo().GetFullName()
		tag := event.GetRelease().GetTagName()
		log.Printf("📥 Release published: %s @ %s", repo, tag)

		// Trigger update for this repository
		go triggerUpdate(repo)
		return nil
	})

	// Register push event handler (for DEV mode - upstream source changes)
	handler.OnPushEventAny(func(ctx context.Context, deliveryID string, eventName string, event *github.PushEvent) error {
		repo := event.GetRepo().GetFullName()
		ref := event.GetRef()
		log.Printf("📥 Push event: %s @ %s", repo, ref)

		// Trigger update for this repository
		go triggerUpdate(repo)
		return nil
	})

	return &Server{
		handler: handler,
	}
}

// HandleWebhook processes incoming webhook requests
func (s *Server) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	err := s.handler.HandleEventRequest(r)
	if err != nil {
		log.Printf("❌ Webhook error: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

// triggerUpdate executes the update workflow for a repository
func triggerUpdate(repo string) {
	// Map repository to subsystem
	subsystem := mapRepoToSubsystem(repo)
	if subsystem == "" {
		log.Printf("⚠️  Unknown repository: %s", repo)
		return
	}

	log.Printf("▶ Triggering update for %s (from repo %s)", subsystem, repo)

	// Call task sync:update with SUBSYSTEM env var
	cmd := exec.Command("task", "sync:update")
	cmd.Env = append(cmd.Env, fmt.Sprintf("SUBSYSTEM=%s", subsystem))

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Update failed for %s: %v\n%s", subsystem, err, output)
		return
	}

	log.Printf("✅ Update completed for %s\n%s", subsystem, output)
}

// mapRepoToSubsystem maps GitHub repository to local subsystem name
func mapRepoToSubsystem(repo string) string {
	repoMap := map[string]string{
		"nats-io/nats-server":            "nats",
		"liftbridge-io/liftbridge":       "liftbridge",
		"influxdata/telegraf":            "telegraf",
		"arcopen/arc":                    "arc",
		"joeblew999/plat-telemetry":      "", // Our own repo - handle releases differently
	}

	return repoMap[repo]
}
