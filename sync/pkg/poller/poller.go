package poller

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/go-github/v80/github"
	"github.com/joeblew99/plat-telemetry/sync/pkg/checker"
)

// RepoConfig holds configuration for checking a repository
type RepoConfig struct {
	Subsystem string
	UseTag    bool   // true = check tag from Taskfile, false = check branch
	Branch    string // branch name if UseTag=false
}

// Poller checks GitHub repositories for updates periodically
type Poller struct {
	client   *github.Client
	interval time.Duration
	repos    map[string]RepoConfig // repo -> config mapping
}

// NewPoller creates a new poller with 1-hour interval
// Set GITHUB_TOKEN env var for authenticated requests (5000/hour vs 60/hour)
func NewPoller() *Poller {
	// Use GitHub token from env if available (increases rate limit to 5000/hour)
	var client *github.Client
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		client = github.NewClient(nil).WithAuthToken(token)
		log.Printf("🔑 Using authenticated GitHub API (5000 req/hour)")
	} else {
		client = github.NewClient(nil)
		log.Printf("⚠️  Using unauthenticated GitHub API (60 req/hour). Set GITHUB_TOKEN for higher limits.")
	}

	return &Poller{
		client:   client,
		interval: 1 * time.Hour, // Reduced from 5min to avoid rate limits
		repos: map[string]RepoConfig{
			"nats-io/nats-server": {
				Subsystem: "nats",
				UseTag:    true, // Version read from Taskfile via config:version task
			},
			"liftbridge-io/liftbridge": {
				Subsystem: "liftbridge",
				UseTag:    false,
				Branch:    "master",
			},
			"influxdata/telegraf": {
				Subsystem: "telegraf",
				UseTag:    false,
				Branch:    "master",
			},
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
	log.Printf("📡 Polling upstream source repositories for new commits...")

	for repo, config := range p.repos {
		log.Printf("   Checking %s (%s)...", repo, config.Subsystem)
		if err := p.checkRepo(repo, config); err != nil {
			log.Printf("   ❌ Failed to check %s: %v", repo, err)
		}
	}
	log.Printf("📡 Polling cycle complete")
}

// checkRepo checks a single repository for updates
func (p *Poller) checkRepo(repo string, config RepoConfig) error {
	ctx := context.Background()

	// Parse repo into owner/name
	owner, repoName := parseRepo(repo)
	if owner == "" || repoName == "" {
		return fmt.Errorf("invalid repo format: %s", repo)
	}

	var latestHash string
	var err error

	if config.UseTag {
		// Get desired version from Taskfile
		tag, err := getDesiredVersion(config.Subsystem)
		if err != nil {
			return fmt.Errorf("failed to get desired version from Taskfile: %w", err)
		}

		// Check specific tag (for repos with pinned versions like NATS)
		log.Printf("   → Fetching tag %s from %s/%s", tag, owner, repoName)
		latestHash, err = p.getTagCommit(ctx, owner, repoName, tag)
		if err != nil {
			return fmt.Errorf("failed to get tag commit: %w", err)
		}
	} else {
		// Check latest commit on branch
		log.Printf("   → Fetching latest commit from %s/%s [%s]", owner, repoName, config.Branch)
		latestHash, err = p.getLatestCommit(ctx, owner, repoName, config.Branch)
		if err != nil {
			return fmt.Errorf("failed to get latest commit: %w", err)
		}
	}

	// Get current version from subsystem
	currentHash, err := checker.GetCurrentVersion(config.Subsystem)
	if err != nil {
		log.Printf("⚠️  Could not read current version for %s: %v", config.Subsystem, err)
		return nil
	}

	// Compare versions
	if latestHash != currentHash {
		log.Printf("   🆕 Update available for %s: %s -> %s", config.Subsystem, currentHash, latestHash)
		log.Printf("   ▶  Triggering rebuild for %s", config.Subsystem)
		go triggerUpdate(config.Subsystem)
	} else {
		log.Printf("   ✅ %s is up to date (%s)", config.Subsystem, currentHash)
	}

	return nil
}

// getTagCommit gets the commit hash for a specific tag
func (p *Poller) getTagCommit(ctx context.Context, owner, repo, tag string) (string, error) {
	// Get the reference for the specific tag
	ref, _, err := p.client.Git.GetRef(ctx, owner, repo, "tags/"+tag)
	if err != nil {
		return "", fmt.Errorf("failed to get tag ref: %w", err)
	}

	// Get commit hash from tag
	commitHash := ref.GetObject().GetSHA()
	if len(commitHash) > 7 {
		commitHash = commitHash[:7]
	}

	return commitHash, nil
}

// getLatestCommit gets the latest commit hash from a branch
func (p *Poller) getLatestCommit(ctx context.Context, owner, repo, branch string) (string, error) {
	commits, _, err := p.client.Repositories.ListCommits(ctx, owner, repo, &github.CommitsListOptions{
		SHA:         branch,
		ListOptions: github.ListOptions{PerPage: 1},
	})
	if err != nil {
		return "", fmt.Errorf("failed to list commits: %w", err)
	}

	if len(commits) == 0 {
		return "", fmt.Errorf("no commits found")
	}

	commitHash := commits[0].GetSHA()
	if len(commitHash) > 7 {
		commitHash = commitHash[:7]
	}

	return commitHash, nil
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

// getDesiredVersion reads the desired version from subsystem Taskfile
func getDesiredVersion(subsystem string) (string, error) {
	// Call task <subsystem>:config:version to get the pinned version
	cmd := exec.Command("task", subsystem+":config:version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run task %s:config:version: %w", subsystem, err)
	}

	// Trim whitespace and return
	version := strings.TrimSpace(string(output))
	if version == "" {
		return "", fmt.Errorf("empty version returned from task %s:config:version", subsystem)
	}

	return version, nil
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
