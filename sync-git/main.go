package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeblew99/plat-telemetry/sync-git/pkg/gitops"
	"github.com/joeblew99/plat-telemetry/sync-git/pkg/version"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "clone":
		clone(args)
	case "pull":
		pull(args)
	case "fetch":
		fetch(args)
	case "checkout":
		checkout(args)
	case "tags":
		tags(args)
	case "version-file":
		versionFile(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`sync-git - Git operations for Taskfile-based builds

Usage: sync-git <command> [arguments]

Commands:
  clone <url> <path> [version]     Clone repository (shallow)
  pull <path>                      Pull updates from origin
  fetch <path> [--tags]            Fetch updates from origin
  checkout <path> <ref>            Checkout tag/branch/commit
  tags <path>                      List all tags
  version-file <path> [options]    Create/read .version file

Version File Options:
  --src=<dir>        Source directory for commit hash
  --version=<tag>    Version tag (e.g., v1.0.0)
  --read             Read existing .version file
  --json             Output as JSON (with --read)`)
}

func clone(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: sync-git clone <url> <path> [version]")
		os.Exit(1)
	}

	url := args[0]
	path := args[1]
	ver := ""
	if len(args) > 2 {
		ver = args[2]
	}

	fmt.Printf("Cloning %s to %s", url, path)
	if ver != "" {
		fmt.Printf(" @ %s", ver)
	}
	fmt.Println()

	if err := gitops.Clone(url, path, ver); err != nil {
		fmt.Fprintf(os.Stderr, "Clone failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Clone completed")
}

func pull(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sync-git pull <path>")
		os.Exit(1)
	}

	path := args[0]

	fmt.Printf("Pulling updates for %s\n", path)

	hash, err := gitops.Pull(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Pull failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Updated to commit %s\n", hash)
}

func fetch(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sync-git fetch <path> [--tags]")
		os.Exit(1)
	}

	path := args[0]
	fetchTags := false
	for _, arg := range args[1:] {
		if arg == "--tags" {
			fetchTags = true
		}
	}

	fmt.Printf("Fetching updates for %s", path)
	if fetchTags {
		fmt.Print(" (with tags)")
	}
	fmt.Println()

	if err := gitops.Fetch(path, fetchTags); err != nil {
		fmt.Fprintf(os.Stderr, "Fetch failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Fetch completed")
}

func checkout(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: sync-git checkout <path> <ref>")
		os.Exit(1)
	}

	path := args[0]
	ref := args[1]

	fmt.Printf("Checking out %s in %s\n", ref, path)

	if err := gitops.Checkout(path, ref); err != nil {
		fmt.Fprintf(os.Stderr, "Checkout failed: %v\n", err)
		os.Exit(1)
	}

	// Show new commit hash
	hash, _ := gitops.GetCommitHash(path)
	fmt.Printf("Checked out %s (commit %s)\n", ref, hash)
}

func tags(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sync-git tags <path>")
		os.Exit(1)
	}

	path := args[0]

	tagList, err := gitops.GetTags(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get tags: %v\n", err)
		os.Exit(1)
	}

	for _, tag := range tagList {
		fmt.Println(tag)
	}
}

func versionFile(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sync-git version-file <path> [options]")
		fmt.Println("Options:")
		fmt.Println("  --src=<dir>        Source directory for commit hash")
		fmt.Println("  --version=<tag>    Version tag (e.g., v1.0.0)")
		fmt.Println("  --read             Read existing .version file")
		fmt.Println("  --json             Output as JSON (with --read)")
		os.Exit(1)
	}

	// Parse path and flags
	path := args[0]
	srcDir := ""
	versionTag := ""
	readMode := false
	jsonOutput := false

	for _, arg := range args[1:] {
		switch {
		case strings.HasPrefix(arg, "--src="):
			srcDir = strings.TrimPrefix(arg, "--src=")
		case strings.HasPrefix(arg, "--version="):
			versionTag = strings.TrimPrefix(arg, "--version=")
		case arg == "--read":
			readMode = true
		case arg == "--json":
			jsonOutput = true
		}
	}

	// Read mode - just read and display
	if readMode {
		readVersionFile(path, jsonOutput)
		return
	}

	// Write mode
	writeVersionFile(path, srcDir, versionTag)
}

func readVersionFile(path string, asJSON bool) {
	// If path is a binary, look for .version next to it
	versionPath := path
	if !strings.HasSuffix(path, ".version") {
		versionPath = filepath.Join(filepath.Dir(path), ".version")
	}

	v, err := version.ReadVersionFile(versionPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read version file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(v.FormatOutput(asJSON))
}

func writeVersionFile(binaryPath, srcDir, versionTag string) {
	// Verify binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Binary not found: %s\n", binaryPath)
		os.Exit(1)
	}

	binDir := filepath.Dir(binaryPath)
	versionPath := filepath.Join(binDir, ".version")

	// If no srcDir provided, check for .src next to .bin
	if srcDir == "" && versionTag == "" {
		subsystemDir := filepath.Dir(binDir)
		possibleSrc := filepath.Join(subsystemDir, ".src")
		if _, err := os.Stat(possibleSrc); err == nil {
			srcDir = possibleSrc
		}
	}

	if err := version.WriteVersionFile(versionPath, binaryPath, srcDir, versionTag); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write version file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created %s\n", versionPath)

	// Show what was written
	v, _ := version.ReadVersionFile(versionPath)
	if v != nil {
		fmt.Println(v.FormatOutput(false))
	}
}
