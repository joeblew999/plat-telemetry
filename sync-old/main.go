package main

import (
	"fmt"
	"os"

	"github.com/joeblew99/plat-telemetry/sync/cmd"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: sync <command> [args]")
		fmt.Println("")
		fmt.Println("Service Commands:")
		fmt.Println("  serve                          Run sync service (webhook server + tunnel)")
		fmt.Println("")
		fmt.Println("GitHub Commands:")
		fmt.Println("  state [repo] [--show]          Capture/display GitHub state")
		fmt.Println("  check                          Check for upstream updates")
		fmt.Println("  poll                           Poll upstream repos for updates")
		fmt.Println("  poll-taskfiles                 Poll Taskfiles for version changes")
		fmt.Println("  webhook                        Start webhook server only")
		fmt.Println("  tunnel <smee-url|new> [target] Forward smee.io webhooks to local")
		fmt.Println("  tunnel-setup <owner/repo>      Create smee channel and configure GitHub webhook")
		fmt.Println("")
		fmt.Println("Cloudflare Commands:")
		fmt.Println("  cf tunnel [port]               Start cloudflared quick tunnel (default: 9090)")
		fmt.Println("  cf poll [interval]             Poll CF audit logs (default: 1m)")
		fmt.Println("  cf webhook [port]              Start CF webhook server only")
		fmt.Println("  cf check                       Check if cloudflared is installed")
		fmt.Println("  cf install                     Install cloudflared")
		fmt.Println("")
		fmt.Println("Git Commands:")
		fmt.Println("  clone <url> <path> [version]   Clone git repository")
		fmt.Println("  pull <path>                    Pull git repository updates")
		fmt.Println("  fetch <path> [--tags]          Fetch updates from origin")
		fmt.Println("  checkout <path> <ref>          Checkout tag/branch/commit")
		fmt.Println("  tags <path>                    List all tags")
		fmt.Println("  version-file <path> [opts]     Create/read .version file for binary")
		fmt.Println("")
		fmt.Println("Environment Variables:")
		fmt.Println("  TUNNEL_TYPE                    Tunnel type: cloudflared, smee, none")
		fmt.Println("  SMEE_URL                       smee.io channel URL")
		fmt.Println("  CF_ACCOUNT_ID                  Cloudflare account ID")
		fmt.Println("  CF_API_TOKEN                   Cloudflare API token")
		fmt.Println("  CF_WEBHOOK_SECRET              Cloudflare webhook secret")
		fmt.Println("  SYNC_ENABLE_CF_AUDIT           Enable CF audit log polling (true/false)")
		fmt.Println("  SYNC_POLL_INTERVAL             Polling interval (e.g., 1m, 5m)")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "serve":
		cmd.Serve()
	case "state":
		cmd.State(os.Args[2:])
	case "check":
		cmd.Check()
	case "poll":
		cmd.Poll()
	case "poll-taskfiles":
		cmd.PollTaskfiles()
	case "webhook":
		cmd.Webhook()
	case "tunnel":
		cmd.Tunnel(os.Args[2:])
	case "tunnel-setup":
		cmd.TunnelSetup(os.Args[2:])
	case "clone":
		cmd.Clone(os.Args[2:])
	case "pull":
		cmd.Pull(os.Args[2:])
	case "fetch":
		cmd.Fetch(os.Args[2:])
	case "checkout":
		cmd.Checkout(os.Args[2:])
	case "tags":
		cmd.Tags(os.Args[2:])
	case "version-file":
		cmd.VersionFile(os.Args[2:])
	case "cf":
		if len(os.Args) < 3 {
			fmt.Println("Usage: sync cf <subcommand> [args]")
			fmt.Println("Subcommands: tunnel, poll, webhook, check, install")
			os.Exit(1)
		}
		subcommand := os.Args[2]
		switch subcommand {
		case "tunnel":
			cmd.CFTunnel(os.Args[3:])
		case "poll":
			cmd.CFPoll(os.Args[3:])
		case "webhook":
			cmd.CFWebhook(os.Args[3:])
		case "check":
			cmd.CFCheck()
		case "install":
			cmd.CFInstall()
		default:
			fmt.Printf("Unknown cf subcommand: %s\n", subcommand)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}
