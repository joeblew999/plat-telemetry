package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/go-github/v80/github"
)

// State represents captured GitHub repository state
type State struct {
	SyncedAt      time.Time     `json:"synced_at"`
	WorkflowRuns  []WorkflowRun `json:"workflow_runs"`
	PagesBuilds   []PagesBuild  `json:"pages_builds"`
	LatestRelease *Release      `json:"latest_release,omitempty"`
}

// WorkflowRun is a simplified workflow run
type WorkflowRun struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	CreatedAt  time.Time `json:"created_at"`
	HTMLURL    string    `json:"html_url"`
}

// PagesBuild is a simplified pages build
type PagesBuild struct {
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	URL       string    `json:"url"`
}

// Release is a simplified release
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
}

// CaptureState fetches current GitHub state for a repository
func CaptureState(owner, repo string) (*State, error) {
	client := github.NewClient(nil)
	ctx := context.Background()

	state := &State{
		SyncedAt: time.Now().UTC(),
	}

	// Capture workflow runs
	runs, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, &github.ListWorkflowRunsOptions{
		ListOptions: github.ListOptions{PerPage: 10},
	})
	if err == nil && runs != nil {
		for _, run := range runs.WorkflowRuns {
			state.WorkflowRuns = append(state.WorkflowRuns, WorkflowRun{
				ID:         run.GetID(),
				Name:       run.GetName(),
				Status:     run.GetStatus(),
				Conclusion: run.GetConclusion(),
				CreatedAt:  run.GetCreatedAt().Time,
				HTMLURL:    run.GetHTMLURL(),
			})
		}
	}

	// Capture pages builds (may not exist)
	builds, _, err := client.Repositories.ListPagesBuilds(ctx, owner, repo, &github.ListOptions{PerPage: 5})
	if err == nil && builds != nil {
		for _, build := range builds {
			state.PagesBuilds = append(state.PagesBuilds, PagesBuild{
				Status:    build.GetStatus(),
				CreatedAt: build.GetCreatedAt().Time,
				URL:       build.GetURL(),
			})
		}
	}

	// Capture latest release (may not exist)
	release, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err == nil && release != nil {
		state.LatestRelease = &Release{
			TagName:     release.GetTagName(),
			Name:        release.GetName(),
			PublishedAt: release.GetPublishedAt().Time,
			HTMLURL:     release.GetHTMLURL(),
		}
	}

	return state, nil
}

// SaveState writes state to a directory
func SaveState(state *State, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state dir: %w", err)
	}

	// Write individual files for easy reading
	if err := writeJSON(filepath.Join(dir, "workflow-runs.json"), state.WorkflowRuns); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(dir, "pages-builds.json"), state.PagesBuilds); err != nil {
		return err
	}
	if state.LatestRelease != nil {
		if err := writeJSON(filepath.Join(dir, "latest-release.json"), state.LatestRelease); err != nil {
			return err
		}
	}
	if err := writeJSON(filepath.Join(dir, "metadata.json"), map[string]string{
		"synced_at": state.SyncedAt.Format(time.RFC3339),
	}); err != nil {
		return err
	}

	return nil
}

// LoadState reads state from a directory
func LoadState(dir string) (*State, error) {
	state := &State{}

	// Read metadata
	metaPath := filepath.Join(dir, "metadata.json")
	if data, err := os.ReadFile(metaPath); err == nil {
		var meta map[string]string
		if json.Unmarshal(data, &meta) == nil {
			if t, err := time.Parse(time.RFC3339, meta["synced_at"]); err == nil {
				state.SyncedAt = t
			}
		}
	}

	// Read workflow runs
	runsPath := filepath.Join(dir, "workflow-runs.json")
	if data, err := os.ReadFile(runsPath); err == nil {
		json.Unmarshal(data, &state.WorkflowRuns)
	}

	// Read pages builds
	buildsPath := filepath.Join(dir, "pages-builds.json")
	if data, err := os.ReadFile(buildsPath); err == nil {
		json.Unmarshal(data, &state.PagesBuilds)
	}

	// Read latest release
	releasePath := filepath.Join(dir, "latest-release.json")
	if data, err := os.ReadFile(releasePath); err == nil {
		state.LatestRelease = &Release{}
		json.Unmarshal(data, state.LatestRelease)
	}

	return state, nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", path, err)
	}
	return os.WriteFile(path, data, 0644)
}
