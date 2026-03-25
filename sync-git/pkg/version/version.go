package version

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joeblew99/plat-telemetry/sync-git/pkg/gitops"
)

// Version represents version metadata for a binary
type Version struct {
	Version   string `yaml:"version,omitempty"`   // For downloaded binaries (e.g., "v2.10.24")
	Commit    string `yaml:"commit,omitempty"`    // For source-built binaries (short hash)
	Timestamp string `yaml:"timestamp,omitempty"` // Build/download time (ISO8601)
	Checksum  string `yaml:"checksum,omitempty"`  // SHA256 of binary
}

// WriteVersionFile creates a .version file for a binary
// If srcDir is provided, it reads the commit from that git repo
// If version string is provided instead, it writes that as the version
func WriteVersionFile(versionPath, binaryPath string, srcDir string, versionTag string) error {
	v := &Version{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Get checksum of the binary
	checksum, err := checksumFile(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to checksum binary: %w", err)
	}
	v.Checksum = checksum

	// If srcDir provided, get commit hash
	if srcDir != "" {
		commit, err := gitops.GetCommitHash(srcDir)
		if err != nil {
			// Fall back to "unknown" if we can't get commit
			commit = "unknown"
		}
		v.Commit = commit
	}

	// If versionTag provided, use it
	if versionTag != "" {
		v.Version = versionTag
	}

	return writeFile(versionPath, v)
}

// WriteVersionFileFromBinary creates .version from binary path
// Assumes binary is at <subsystem>/.bin/<name> and source at <subsystem>/.src/
func WriteVersionFileFromBinary(binaryPath string) error {
	binDir := filepath.Dir(binaryPath)
	versionPath := filepath.Join(binDir, ".version")

	// Check if .src exists next to .bin
	subsystemDir := filepath.Dir(binDir)
	srcDir := filepath.Join(subsystemDir, ".src")

	// If .src exists, use it for commit hash
	if _, err := os.Stat(srcDir); err == nil {
		return WriteVersionFile(versionPath, binaryPath, srcDir, "")
	}

	// No .src - just create with timestamp and checksum
	return WriteVersionFile(versionPath, binaryPath, "", "")
}

// ReadVersionFile parses a .version file
func ReadVersionFile(path string) (*Version, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read version file: %w", err)
	}

	v := &Version{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "version":
			v.Version = value
		case "commit":
			v.Commit = value
		case "timestamp":
			v.Timestamp = value
		case "checksum":
			v.Checksum = value
		}
	}

	return v, nil
}

// checksumFile computes SHA256 of a file
func checksumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// writeFile writes version info to a file
func writeFile(path string, v *Version) error {
	var lines []string

	// Write fields in order (matching current shell output)
	if v.Version != "" {
		lines = append(lines, fmt.Sprintf("version: %s", v.Version))
	}
	if v.Commit != "" {
		lines = append(lines, fmt.Sprintf("commit: %s", v.Commit))
	}
	if v.Timestamp != "" {
		lines = append(lines, fmt.Sprintf("timestamp: %s", v.Timestamp))
	}
	if v.Checksum != "" {
		lines = append(lines, fmt.Sprintf("checksum: %s", v.Checksum))
	}

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0644)
}

// FormatOutput formats version for display
func (v *Version) FormatOutput(asJSON bool) string {
	if asJSON {
		// Simple JSON output
		var parts []string
		if v.Version != "" {
			parts = append(parts, fmt.Sprintf(`"version":"%s"`, v.Version))
		}
		if v.Commit != "" {
			parts = append(parts, fmt.Sprintf(`"commit":"%s"`, v.Commit))
		}
		if v.Timestamp != "" {
			parts = append(parts, fmt.Sprintf(`"timestamp":"%s"`, v.Timestamp))
		}
		if v.Checksum != "" {
			parts = append(parts, fmt.Sprintf(`"checksum":"%s"`, v.Checksum))
		}
		return "{" + strings.Join(parts, ",") + "}"
	}

	// Plain text output
	var lines []string
	if v.Version != "" {
		lines = append(lines, fmt.Sprintf("version: %s", v.Version))
	}
	if v.Commit != "" {
		lines = append(lines, fmt.Sprintf("commit: %s", v.Commit))
	}
	if v.Timestamp != "" {
		lines = append(lines, fmt.Sprintf("timestamp: %s", v.Timestamp))
	}
	if v.Checksum != "" {
		lines = append(lines, fmt.Sprintf("checksum: %s", v.Checksum))
	}
	return strings.Join(lines, "\n")
}
