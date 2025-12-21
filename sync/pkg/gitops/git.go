package gitops

import (
	"fmt"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Clone clones a repository to the specified path at a specific version/branch
func Clone(url, path, version string) error {
	opts := &git.CloneOptions{
		URL:   url,
		Depth: 1,
	}

	// If version is specified, clone at that reference
	if version != "" {
		opts.ReferenceName = plumbing.ReferenceName(version)
	}

	_, err := git.PlainClone(path, false, opts)
	if err != nil {
		return fmt.Errorf("failed to clone %s: %w", url, err)
	}

	return nil
}

// Pull updates the repository at the specified path and returns the new commit hash
func Pull(path string) (string, error) {
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
	return GetCommitHash(path)
}

// GetCommitHash returns the short commit hash of HEAD
func GetCommitHash(path string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", fmt.Errorf("failed to open repo: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Return short hash (first 7 characters)
	hash := head.Hash().String()
	if len(hash) > 7 {
		hash = hash[:7]
	}

	return hash, nil
}

// GetCommitHashFromBinary returns the commit hash from a binary's parent .src directory
// This assumes the binary is in <subsystem>/.bin/ and source is in <subsystem>/.src/
func GetCommitHashFromBinary(binPath string) (string, error) {
	// Get parent directory (subsystem dir)
	subsystemDir := filepath.Dir(filepath.Dir(binPath))
	srcDir := filepath.Join(subsystemDir, ".src")

	return GetCommitHash(srcDir)
}
