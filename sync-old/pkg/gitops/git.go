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

	// Return short hash (first 8 characters to match git rev-parse --short)
	hash := head.Hash().String()
	if len(hash) > 8 {
		hash = hash[:8]
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

// Fetch fetches updates from origin, optionally including tags
func Fetch(path string, tags bool) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("failed to open repo: %w", err)
	}

	opts := &git.FetchOptions{
		RemoteName: "origin",
	}

	if tags {
		opts.Tags = git.AllTags
	}

	err = repo.Fetch(opts)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	return nil
}

// Checkout checks out a specific reference (tag, branch, or commit)
func Checkout(path, ref string) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("failed to open repo: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Try to resolve as a tag first
	hash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		// Try as a remote branch
		hash, err = repo.ResolveRevision(plumbing.Revision("origin/" + ref))
		if err != nil {
			return fmt.Errorf("failed to resolve ref %s: %w", ref, err)
		}
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Hash: *hash,
	})
	if err != nil {
		return fmt.Errorf("failed to checkout: %w", err)
	}

	return nil
}

// GetTags returns a list of all tags in the repository
func GetTags(path string) ([]string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open repo: %w", err)
	}

	tagsIter, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}

	var tags []string
	err = tagsIter.ForEach(func(ref *plumbing.Reference) error {
		// Get just the tag name (remove refs/tags/ prefix)
		name := ref.Name().Short()
		tags = append(tags, name)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate tags: %w", err)
	}

	return tags, nil
}

// GetBranch returns the current branch name
func GetBranch(path string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", fmt.Errorf("failed to open repo: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	if !head.Name().IsBranch() {
		return "", fmt.Errorf("HEAD is not a branch (detached head)")
	}

	return head.Name().Short(), nil
}
