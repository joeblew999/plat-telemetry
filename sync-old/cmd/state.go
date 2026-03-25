package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joeblew99/plat-telemetry/sync/pkg/github"
)

// State captures or displays GitHub repository state
func State(args []string) {
	// Default repo from env or flag
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
		fmt.Println("Usage: sync state [owner/repo] [--dir=.github/state] [--show]")
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
		log.Fatalf("❌ Invalid repo format: %s (expected owner/repo)", repo)
	}

	log.Printf("▶ Capturing state for %s...", repo)

	state, err := github.CaptureState(parts[0], parts[1])
	if err != nil {
		log.Fatalf("❌ Failed to capture state: %v", err)
	}

	if err := github.SaveState(state, stateDir); err != nil {
		log.Fatalf("❌ Failed to save state: %v", err)
	}

	log.Printf("✅ State captured:")
	log.Printf("  - Workflow runs: %d entries", len(state.WorkflowRuns))
	log.Printf("  - Pages builds: %d entries", len(state.PagesBuilds))
	if state.LatestRelease != nil {
		log.Printf("  - Latest release: %s", state.LatestRelease.TagName)
	} else {
		log.Printf("  - Latest release: none")
	}
}

func showState(stateDir string) {
	state, err := github.LoadState(stateDir)
	if err != nil {
		log.Fatalf("❌ Failed to load state: %v", err)
	}

	if state.SyncedAt.IsZero() {
		fmt.Println("❌ No state found. Run 'sync state <repo>' to capture.")
		os.Exit(1)
	}

	fmt.Println("=== GitHub State ===")
	fmt.Printf("Last synced: %s\n\n", state.SyncedAt.Format("2006-01-02 15:04:05 UTC"))

	fmt.Println("--- Workflow Runs ---")
	if len(state.WorkflowRuns) == 0 {
		fmt.Println("No data")
	} else {
		for _, run := range state.WorkflowRuns {
			conclusion := run.Conclusion
			if conclusion == "" {
				conclusion = run.Status
			}
			fmt.Printf("%s | %s | %s\n", conclusion, run.Name, run.CreatedAt.Format("2006-01-02 15:04"))
		}
	}
	fmt.Println()

	fmt.Println("--- Pages Builds ---")
	if len(state.PagesBuilds) == 0 {
		fmt.Println("No data")
	} else {
		for _, build := range state.PagesBuilds {
			fmt.Printf("%s | %s\n", build.Status, build.CreatedAt.Format("2006-01-02 15:04"))
		}
	}
	fmt.Println()

	fmt.Println("--- Latest Release ---")
	if state.LatestRelease == nil {
		fmt.Println("No data")
	} else {
		fmt.Printf("%s | %s\n", state.LatestRelease.TagName, state.LatestRelease.PublishedAt.Format("2006-01-02 15:04"))
	}
}
