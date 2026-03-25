package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeblew99/plat-telemetry/sync/pkg/version"
)

// VersionFile handles version-file command
// Usage:
//
//	sync version-file <path>                       Write .version for binary at path
//	sync version-file <path> --src=<srcdir>        Use specific source dir for commit
//	sync version-file <path> --version=v1.0.0      Use version tag instead of commit
//	sync version-file <path> --read                Read and display existing .version
//	sync version-file <path> --json                Output as JSON (for --read)
func VersionFile(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sync version-file <path> [options]")
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
		fmt.Fprintf(os.Stderr, "❌ Failed to read version file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(v.FormatOutput(asJSON))
}

func writeVersionFile(binaryPath, srcDir, versionTag string) {
	// Verify binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "❌ Binary not found: %s\n", binaryPath)
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
		fmt.Fprintf(os.Stderr, "❌ Failed to write version file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Created %s\n", versionPath)

	// Show what was written
	v, _ := version.ReadVersionFile(versionPath)
	if v != nil {
		fmt.Println(v.FormatOutput(false))
	}
}
