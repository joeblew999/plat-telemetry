package main

import (
	"fmt"
	"os"

	"github.com/joeblew99/plat-telemetry/sync/cmd"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: sync <command> [args]")
		fmt.Println("Commands:")
		fmt.Println("  check                          Check for upstream updates")
		fmt.Println("  poll                           Poll upstream repos for updates")
		fmt.Println("  watch                          Start webhook server")
		fmt.Println("  clone <url> <path> [version]   Clone git repository")
		fmt.Println("  pull <path>                    Pull git repository updates")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "check":
		cmd.Check()
	case "poll":
		cmd.Poll()
	case "watch":
		cmd.Watch()
	case "clone":
		cmd.Clone(os.Args[2:])
	case "pull":
		cmd.Pull(os.Args[2:])
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}
