package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kardianos/service"
)

type program struct {
	cmd     *exec.Cmd
	workDir string
}

func (p *program) Start(s service.Service) error {
	log.Println("Starting plat-telemetry service...")
	go p.run()
	return nil
}

func (p *program) run() {
	// Use full path since launchd doesn't have Homebrew in PATH
	taskPath := "/opt/homebrew/bin/task"
	if _, err := os.Stat(taskPath); os.IsNotExist(err) {
		// Fallback for Intel Macs or custom installs
		taskPath = "/usr/local/bin/task"
	}

	p.cmd = exec.Command(taskPath, "start:fg")
	p.cmd.Dir = p.workDir
	p.cmd.Stdout = os.Stdout
	p.cmd.Stderr = os.Stderr

	// Set PATH so child processes (task calling task) can find binaries
	p.cmd.Env = append(os.Environ(),
		"PATH=/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin",
	)

	if err := p.cmd.Run(); err != nil {
		log.Printf("Task exited: %v", err)
	}
}

func (p *program) Stop(s service.Service) error {
	log.Println("Stopping plat-telemetry service...")
	if p.cmd != nil && p.cmd.Process != nil {
		// Send SIGTERM to the process group
		p.cmd.Process.Signal(os.Interrupt)
	}
	return nil
}

func main() {
	// Get the directory where the binary lives (project root)
	exe, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	// service binary is in service/.bin/, so go up 2 levels
	workDir := filepath.Dir(filepath.Dir(filepath.Dir(exe)))

	svcConfig := &service.Config{
		Name:             "plat-telemetry",
		DisplayName:      "Plat Telemetry Service",
		Description:      "Runs plat-telemetry via Process Compose",
		WorkingDirectory: workDir,
		Option: service.KeyValue{
			"UserService": true, // Install as user service (LaunchAgent, not LaunchDaemon)
		},
	}

	prg := &program{workDir: workDir}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			err = s.Install()
			if err != nil {
				log.Fatalf("Failed to install: %v", err)
			}
			log.Println("Service installed")
			return
		case "uninstall":
			err = s.Uninstall()
			if err != nil {
				log.Fatalf("Failed to uninstall: %v", err)
			}
			log.Println("Service uninstalled")
			return
		case "start":
			err = s.Start()
			if err != nil {
				log.Fatalf("Failed to start: %v", err)
			}
			log.Println("Service started")
			return
		case "stop":
			err = s.Stop()
			if err != nil {
				log.Fatalf("Failed to stop: %v", err)
			}
			log.Println("Service stopped")
			return
		case "status":
			status, err := s.Status()
			if err != nil {
				log.Fatalf("Failed to get status: %v", err)
			}
			switch status {
			case service.StatusRunning:
				log.Println("Service is running")
			case service.StatusStopped:
				log.Println("Service is stopped")
			default:
				log.Println("Service status unknown")
			}
			return
		}
	}

	// Run as service
	err = s.Run()
	if err != nil {
		log.Fatal(err)
	}
}
