package cmd

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Clone clones a git repository (thin wrapper calling gitops logic)
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

	err := cloneRepo(url, path, version)
	if err != nil {
		fmt.Printf("❌ Clone failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Clone completed")
}

// Pull updates a git repository (thin wrapper calling gitops logic)
func Pull(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sync pull <path>")
		os.Exit(1)
	}

	path := args[0]

	fmt.Printf("▶ Pulling updates for %s\n", path)

	hash, err := pullRepo(path)
	if err != nil {
		fmt.Printf("❌ Pull failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Updated to commit %s\n", hash)
}

// cloneRepo contains the git clone logic
func cloneRepo(url, path, version string) error {
	opts := &git.CloneOptions{
		URL:   url,
		Depth: 1,
	}

	if version != "" {
		opts.ReferenceName = plumbing.ReferenceName(version)
	}

	_, err := git.PlainClone(path, false, opts)
	if err != nil {
		return fmt.Errorf("failed to clone %s: %w", url, err)
	}

	return nil
}

// pullRepo contains the git pull logic and returns the new commit hash
func pullRepo(path string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", fmt.Errorf("failed to open repo: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	err = worktree.Pull(&git.PullOptions{
		RemoteName: "origin",
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return "", fmt.Errorf("failed to pull: %w", err)
	}

	// Get and return new commit hash
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	hash := head.Hash().String()
	if len(hash) > 7 {
		hash = hash[:7]
	}

	return hash, nil
}
