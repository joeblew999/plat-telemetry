package taskfilepoller

import (
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// TaskfilePoller monitors Taskfiles for version changes
type TaskfilePoller struct {
	interval   time.Duration
	subsystems []string
	versions   map[string]string // subsystem -> last known version
}

// NewTaskfilePoller creates a new Taskfile poller
func NewTaskfilePoller() *TaskfilePoller {
	return &TaskfilePoller{
		interval: 30 * time.Second, // Check every 30 seconds
		versions: make(map[string]string),
	}
}

// discoverSubsystems finds all subsystems that have config:version task
func (p *TaskfilePoller) discoverSubsystems() ([]string, error) {
	cmd := exec.Command("task", "--list-all")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Match patterns like "* nats:config:version:" or "nats:config:version"
	re := regexp.MustCompile(`\* (\w+):config:version:`)
	matches := re.FindAllStringSubmatch(string(output), -1)

	var subsystems []string
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			subsystems = append(subsystems, match[1])
			seen[match[1]] = true
		}
	}

	return subsystems, nil
}

// Start begins the polling loop
func (p *TaskfilePoller) Start() error {
	log.Printf("🔄 Starting Taskfile poller (interval: %v)", p.interval)

	// Discover subsystems dynamically
	subsystems, err := p.discoverSubsystems()
	if err != nil {
		return err
	}
	p.subsystems = subsystems
	log.Printf("   Discovered %d subsystems with config:version: %v", len(subsystems), subsystems)

	// Initialize current versions
	for _, subsystem := range p.subsystems {
		version, err := p.getTaskfileVersion(subsystem)
		if err != nil {
			log.Printf("⚠️  Could not read initial version for %s: %v", subsystem, err)
			continue
		}
		p.versions[subsystem] = version
		log.Printf("   %s: %s", subsystem, version)
	}

	// Poll on interval
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for range ticker.C {
		p.checkAll()
	}

	return nil
}

// checkAll checks all subsystem Taskfiles for version changes
func (p *TaskfilePoller) checkAll() {
	log.Printf("📝 Checking Taskfiles for version changes...")

	for _, subsystem := range p.subsystems {
		if err := p.checkSubsystem(subsystem); err != nil {
			log.Printf("   ❌ Failed to check %s: %v", subsystem, err)
		}
	}
}

// checkSubsystem checks a single subsystem Taskfile
func (p *TaskfilePoller) checkSubsystem(subsystem string) error {
	// Get current version from Taskfile
	currentVersion, err := p.getTaskfileVersion(subsystem)
	if err != nil {
		return err
	}

	// Get last known version
	lastVersion := p.versions[subsystem]

	// Compare
	if currentVersion != lastVersion {
		log.Printf("   🆕 Taskfile version changed for %s: %s -> %s", subsystem, lastVersion, currentVersion)
		log.Printf("   ▶  Triggering rebuild for %s", subsystem)

		// Update stored version
		p.versions[subsystem] = currentVersion

		// Trigger update workflow
		go p.triggerUpdate(subsystem)
	}

	return nil
}

// getTaskfileVersion reads the version from subsystem Taskfile
func (p *TaskfilePoller) getTaskfileVersion(subsystem string) (string, error) {
	cmd := exec.Command("task", subsystem+":config:version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	version := strings.TrimSpace(string(output))
	return version, nil
}

// triggerUpdate executes the update workflow for a subsystem
func (p *TaskfilePoller) triggerUpdate(subsystem string) {
	log.Printf("▶ Triggering update for %s (Taskfile version changed)", subsystem)

	cmd := exec.Command("task", "sync:update")
	cmd.Env = append(os.Environ(), "SUBSYSTEM="+subsystem)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Update failed for %s: %v\n%s", subsystem, err, output)
		return
	}

	log.Printf("✅ Update completed for %s", subsystem)
}
