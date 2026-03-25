package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joeblew99/plat-telemetry/sync-gh/pkg/checker"
	"github.com/joeblew99/plat-telemetry/sync-gh/pkg/github"
	"github.com/joeblew99/plat-telemetry/sync-gh/pkg/poller"
	"github.com/joeblew99/plat-telemetry/sync-gh/pkg/taskfilepoller"
	"github.com/joeblew99/plat-telemetry/sync-gh/pkg/tunnel"
	"github.com/joeblew99/plat-telemetry/sync-gh/pkg/webhook"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "state":
		state(args)
	case "check":
		check()
	case "poll":
		poll()
	case "poll-taskfiles":
		pollTaskfiles()
	case "webhook":
		runWebhook()
	case "tunnel":
		runTunnel(args)
	case "tunnel-setup":
		tunnelSetup(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`sync-gh - GitHub integration for Taskfile-based builds

Usage: sync-gh <command> [arguments]

Commands:
  state [repo] [--show] [--dir=DIR]   Capture/display GitHub state
  check                                Check for upstream updates
  poll                                 Poll upstream repos for updates
  poll-taskfiles                       Poll Taskfiles for version changes
  webhook                              Start webhook server only
  tunnel <smee-url|new> [target]       Forward smee.io webhooks to local
  tunnel-setup <owner/repo> [--events=...] Create smee channel and configure webhook

Environment:
  PORT             Webhook server port (default: 8080)
  GITHUB_TOKEN     GitHub token for API (increases rate limit)
  GITHUB_REPOSITORY Default repository for state command`)
}

func state(args []string) {
	repo := os.Getenv("GITHUB_REPOSITORY")
	stateDir := ".github/state"
	showOnly := false

	for _, arg := range args {
		if strings.HasPrefix(arg, "--repo=") {
			repo = strings.TrimPrefix(arg, "--repo=")
		} else if strings.HasPrefix(arg, "--dir=") {
			stateDir = strings.TrimPrefix(arg, "--dir=")
		} else if arg == "--show" {
			showOnly = true
		} else if !strings.HasPrefix(arg, "-") && repo == "" {
			repo = arg
		}
	}

	if showOnly {
		showState(stateDir)
		return
	}

	if repo == "" {
		fmt.Println("Usage: sync-gh state [owner/repo] [--dir=.github/state] [--show]")
		fmt.Println("  Captures GitHub state (workflow runs, pages builds, releases)")
		fmt.Println("  --show    Display current state without fetching")
		fmt.Println("  --dir     State directory (default: .github/state)")
		fmt.Println("")
		fmt.Println("Environment:")
		fmt.Println("  GITHUB_REPOSITORY  Default repository")
		os.Exit(1)
	}

	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		log.Fatalf("Invalid repo format: %s (expected owner/repo)", repo)
	}

	log.Printf("Capturing state for %s...", repo)

	s, err := github.CaptureState(parts[0], parts[1])
	if err != nil {
		log.Fatalf("Failed to capture state: %v", err)
	}

	if err := github.SaveState(s, stateDir); err != nil {
		log.Fatalf("Failed to save state: %v", err)
	}

	log.Printf("State captured:")
	log.Printf("  - Workflow runs: %d entries", len(s.WorkflowRuns))
	log.Printf("  - Pages builds: %d entries", len(s.PagesBuilds))
	if s.LatestRelease != nil {
		log.Printf("  - Latest release: %s", s.LatestRelease.TagName)
	} else {
		log.Printf("  - Latest release: none")
	}
}

func showState(stateDir string) {
	s, err := github.LoadState(stateDir)
	if err != nil {
		log.Fatalf("Failed to load state: %v", err)
	}

	if s.SyncedAt.IsZero() {
		fmt.Println("No state found. Run 'sync-gh state <repo>' to capture.")
		os.Exit(1)
	}

	fmt.Println("=== GitHub State ===")
	fmt.Printf("Last synced: %s\n\n", s.SyncedAt.Format("2006-01-02 15:04:05 UTC"))

	fmt.Println("--- Workflow Runs ---")
	if len(s.WorkflowRuns) == 0 {
		fmt.Println("No data")
	} else {
		for _, run := range s.WorkflowRuns {
			conclusion := run.Conclusion
			if conclusion == "" {
				conclusion = run.Status
			}
			fmt.Printf("%s | %s | %s\n", conclusion, run.Name, run.CreatedAt.Format("2006-01-02 15:04"))
		}
	}
	fmt.Println()

	fmt.Println("--- Pages Builds ---")
	if len(s.PagesBuilds) == 0 {
		fmt.Println("No data")
	} else {
		for _, build := range s.PagesBuilds {
			fmt.Printf("%s | %s\n", build.Status, build.CreatedAt.Format("2006-01-02 15:04"))
		}
	}
	fmt.Println()

	fmt.Println("--- Latest Release ---")
	if s.LatestRelease == nil {
		fmt.Println("No data")
	} else {
		fmt.Printf("%s | %s\n", s.LatestRelease.TagName, s.LatestRelease.PublishedAt.Format("2006-01-02 15:04"))
	}
}

func check() {
	// Check all configured subsystems
	subsystems := []string{"nats", "liftbridge", "telegraf"}

	for _, subsystem := range subsystems {
		current, latest, err := checker.CheckVersion(subsystem)
		if err != nil {
			log.Printf("%s: error - %v", subsystem, err)
			continue
		}

		if current != latest {
			log.Printf("%s: update available %s -> %s", subsystem, current, latest)
		} else {
			log.Printf("%s: up to date (%s)", subsystem, current)
		}
	}
}

func poll() {
	p := poller.NewPoller()
	if err := p.Start(); err != nil {
		log.Fatal(err)
	}
}

func pollTaskfiles() {
	p := taskfilepoller.NewTaskfilePoller()
	if err := p.Start(); err != nil {
		log.Fatal(err)
	}
}

func runWebhook() {
	port := os.Getenv("PORT")
	webhook.Run(port)
}

func runTunnel(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sync-gh tunnel <smee-url|new> [target]")
		fmt.Println("  smee-url: Your smee.io channel URL (e.g., https://smee.io/abc123)")
		fmt.Println("  new:      Auto-generate a new smee channel")
		fmt.Println("  target:   Local target URL (default: http://localhost:9090/webhook)")
		os.Exit(1)
	}

	smeeURL := args[0]
	target := "http://localhost:9090/webhook"
	if len(args) > 1 {
		target = args[1]
	}

	if smeeURL == "new" {
		smeeURL = tunnel.GenerateChannel()
		log.Printf("Created channel: %s", smeeURL)
	}

	tunnel.Run(smeeURL, target)
}

func tunnelSetup(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sync-gh tunnel-setup <owner/repo> [--events=push,release,workflow_run]")
		fmt.Println("  Creates a smee channel and configures GitHub webhook")
		os.Exit(1)
	}

	repo := args[0]
	events := "push,release,workflow_run,page_build,deployment_status"

	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "--events=") {
			events = strings.TrimPrefix(arg, "--events=")
		}
	}

	smeeURL := tunnel.GenerateChannel()
	log.Printf("Created smee channel: %s", smeeURL)

	if err := tunnel.ConfigureGitHubWebhook(repo, smeeURL, events); err != nil {
		log.Fatalf("Failed to configure webhook: %v", err)
	}

	log.Printf("Webhook configured for %s", repo)
	log.Printf("")
	log.Printf("To start receiving webhooks:")
	log.Printf("  sync-gh tunnel %s", smeeURL)
}
