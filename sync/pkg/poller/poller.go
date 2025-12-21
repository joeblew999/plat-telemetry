package poller

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/google/go-github/v80/github"
	"github.com/joeblew99/plat-telemetry/sync/pkg/checker"
)

// Poller checks GitHub repositories for updates periodically
type Poller struct {
	client   *github.Client
	interval time.Duration
	repos    map[string]string // repo -> subsystem mapping
}

// NewPoller creates a new poller with 5-minute interval
func NewPoller() *Poller {
	return &Poller{
		client:   github.NewClient(nil),
		interval: 5 * time.Minute,
		repos: map[string]string{
			"nats-io/nats-server":      "nats",
			"liftbridge-io/liftbridge": "liftbridge",
			"influxdata/telegraf":      "telegraf",
			// arc repo doesn't exist publicly, commented out
			// "arcopen/arc":              "arc",
		},
	}
}

// Start begins the polling loop
func (p *Poller) Start() error {
	log.Printf("🔄 Starting poller (interval: %v)", p.interval)

	// Do initial check immediately
	p.checkAll()

	// Then poll on interval
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for range ticker.C {
		p.checkAll()
	}

	return nil
}

// checkAll checks all upstream repositories for updates
func (p *Poller) checkAll() {
	log.Printf("📡 Polling upstream repositories...")

	for repo, subsystem := range p.repos {
		if err := p.checkRepo(repo, subsystem); err != nil {
			log.Printf("❌ Failed to check %s: %v", repo, err)
		}
	}
}

// checkRepo checks a single repository for updates
func (p *Poller) checkRepo(repo, subsystem string) error {
	ctx := context.Background()

	// Parse repo into owner/name
	owner, repoName := parseRepo(repo)
	if owner == "" || repoName == "" {
		return fmt.Errorf("invalid repo format: %s", repo)
	}

	// Get latest commit from default branch
	commits, _, err := p.client.Repositories.ListCommits(ctx, owner, repoName, &github.CommitsListOptions{
		ListOptions: github.ListOptions{PerPage: 1},
	})
	if err != nil {
		return fmt.Errorf("failed to list commits: %w", err)
	}

	if len(commits) == 0 {
		return fmt.Errorf("no commits found")
	}

	latestHash := commits[0].GetSHA()
	if len(latestHash) > 7 {
		latestHash = latestHash[:7]
	}

	// Get current version from subsystem
	currentHash, err := checker.GetCurrentVersion(subsystem)
	if err != nil {
		log.Printf("⚠️  Could not read current version for %s: %v", subsystem, err)
		return nil
	}

	// Compare versions
	if latestHash != currentHash {
		log.Printf("🆕 Update available for %s: %s -> %s", subsystem, currentHash, latestHash)
		go triggerUpdate(subsystem)
	} else {
		log.Printf("✅ %s is up to date (%s)", subsystem, currentHash)
	}

	return nil
}

// parseRepo splits "owner/repo" into (owner, repo)
func parseRepo(repo string) (string, string) {
	// Simple split on "/"
	for i, c := range repo {
		if c == '/' {
			return repo[:i], repo[i+1:]
		}
	}
	return "", ""
}

// triggerUpdate executes the update workflow for a subsystem
func triggerUpdate(subsystem string) {
	log.Printf("▶ Triggering update for %s", subsystem)

	// Call task sync:update with SUBSYSTEM env var
	cmd := exec.Command("task", "sync:update")
	cmd.Env = append(os.Environ(), fmt.Sprintf("SUBSYSTEM=%s", subsystem))

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Update failed for %s: %v\n%s", subsystem, err, output)
		return
	}

	log.Printf("✅ Update completed for %s\n%s", subsystem, output)
}
