package cmd

import (
	"fmt"
	"os"

	"github.com/joeblew99/plat-telemetry/sync/pkg/gitops"
)

// Clone clones a git repository
func Clone(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: sync clone <url> <path> [version]")
		os.Exit(1)
	}

	url := args[0]
	path := args[1]
	version := ""
	if len(args) > 2 {
		version = args[2]
	}

	fmt.Printf("▶ Cloning %s to %s", url, path)
	if version != "" {
		fmt.Printf(" @ %s", version)
	}
	fmt.Println()

	if err := gitops.Clone(url, path, version); err != nil {
		fmt.Printf("❌ Clone failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Clone completed")
}

// Pull updates a git repository
func Pull(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sync pull <path>")
		os.Exit(1)
	}

	path := args[0]

	fmt.Printf("▶ Pulling updates for %s\n", path)

	hash, err := gitops.Pull(path)
	if err != nil {
		fmt.Printf("❌ Pull failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Updated to commit %s\n", hash)
}

// Fetch fetches updates from origin
func Fetch(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sync fetch <path> [--tags]")
		os.Exit(1)
	}

	path := args[0]
	tags := false
	for _, arg := range args[1:] {
		if arg == "--tags" {
			tags = true
		}
	}

	fmt.Printf("▶ Fetching updates for %s", path)
	if tags {
		fmt.Print(" (with tags)")
	}
	fmt.Println()

	if err := gitops.Fetch(path, tags); err != nil {
		fmt.Printf("❌ Fetch failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Fetch completed")
}

// Checkout checks out a specific reference
func Checkout(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: sync checkout <path> <ref>")
		os.Exit(1)
	}

	path := args[0]
	ref := args[1]

	fmt.Printf("▶ Checking out %s in %s\n", ref, path)

	if err := gitops.Checkout(path, ref); err != nil {
		fmt.Printf("❌ Checkout failed: %v\n", err)
		os.Exit(1)
	}

	// Show new commit hash
	hash, _ := gitops.GetCommitHash(path)
	fmt.Printf("✅ Checked out %s (commit %s)\n", ref, hash)
}

// Tags lists all tags in a repository
func Tags(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sync tags <path>")
		os.Exit(1)
	}

	path := args[0]

	tags, err := gitops.GetTags(path)
	if err != nil {
		fmt.Printf("❌ Failed to get tags: %v\n", err)
		os.Exit(1)
	}

	for _, tag := range tags {
		fmt.Println(tag)
	}
}
