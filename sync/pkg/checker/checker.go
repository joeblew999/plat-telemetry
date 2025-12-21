package checker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CheckVersion checks if a subsystem has updates available
// Returns: current version, latest version, error
func CheckVersion(subsystem string) (string, string, error) {
	// Task always runs from root directory, so paths are relative to cwd
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// Read current version from <subsystem>/.bin/.version
	versionPath := filepath.Join(cwd, subsystem, ".bin", ".version")
	current, err := readVersion(versionPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read current version: %w", err)
	}

	// For now, return same version (TODO: implement GitHub API check)
	// This will be expanded to check GitHub releases API
	latest := current

	return current, latest, nil
}

// readVersion reads the version file and extracts the commit hash
func readVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "commit:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	return "", fmt.Errorf("no commit hash found in version file")
}

// GetCurrentVersion gets the current commit hash for a subsystem
func GetCurrentVersion(subsystem string) (string, error) {
	// Task always runs from root directory, so paths are relative to cwd
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// Read current version from <subsystem>/.bin/.version
	versionPath := filepath.Join(cwd, subsystem, ".bin", ".version")
	return readVersion(versionPath)
}
