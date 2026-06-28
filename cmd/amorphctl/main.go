// Package main implements the AmorphDB control tool
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

)

func main() {
	if len(os.Args) < 2 {
		showHelp()
		os.Exit(1)
	}

	command := os.Args[1]

	// Show help without requiring service connection
	if command == "help" || command == "--help" || command == "-h" {
		showHelp()
		return
	}

	// init-pwa is a local file-system operation; it does not need to
	// connect to the amorphd service.
	if command == "init-pwa" {
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: %s init-pwa <appname> <directory>\n", os.Args[0])
			os.Exit(1)
		}
		if err := handleInitPWA(os.Args[2], os.Args[3]); err != nil {
			fmt.Fprintf(os.Stderr, "Command failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Get local socket path - check environment variable first
	socketPath := os.Getenv("AMORPH_SOCKET")
	if socketPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get user home directory: %v\n", err)
			os.Exit(1)
		}
		socketPath = filepath.Join(homeDir, ".amorph", "socket")
	}

	// Create client connection to local socket only
	client, err := NewControlClient(socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to AmorphDB service: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure the AmorphDB service (amorphd) is running.\n")
		os.Exit(1)
	}
	defer client.Close()

	// Execute command
	switch command {
	case "status":
		err = handleStatus(client)
	case "stop":
		err = handleStop(client)
	case "compact":
		err = handleCompact(client)
	case "zones":
		if len(os.Args) < 3 || os.Args[2] != "list" {
			fmt.Fprintf(os.Stderr, "Usage: %s zones list\n", os.Args[0])
			os.Exit(1)
		}
		err = handleZonesList(client)
	case "create-mesh":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: %s create-mesh <name>\n", os.Args[0])
			os.Exit(1)
		}
		err = handleCreateMesh(client, os.Args[2])
	case "join":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: %s join <address>\n", os.Args[0])
			os.Exit(1)
		}
		err = handleJoin(client, os.Args[2])
	case "bridge":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: %s bridge <address>\n", os.Args[0])
			os.Exit(1)
		}
		err = handleBridge(client, os.Args[2])
	case "detach":
		meshName := ""
		if len(os.Args) >= 3 {
			meshName = os.Args[2]
		}
		err = handleDetach(client, meshName)
	case "extract":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: %s extract <path> [-o output_file]\n", os.Args[0])
			os.Exit(1)
		}
		err = handleExtract(client, os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		fmt.Fprintf(os.Stderr, "Run '%s help' for usage information.\n", os.Args[0])
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Command failed: %v\n", err)
		os.Exit(1)
	}
}

func handleStatus(client *ControlClient) error {
	status, err := client.GetStatus()
	if err != nil {
		return err
	}

	fmt.Printf("Service: running\n")
	fmt.Printf("Uptime: %s\n", formatDuration(status.Uptime))
	fmt.Printf("Node Identity: %s\n", status.NodeIdentity)
	fmt.Printf("Local Socket: %s\n", status.LocalSocket)
	fmt.Printf("Network Socket: *:%d\n", status.NetworkPort)
	fmt.Printf("Active Connections: %d\n", status.Connections)
	fmt.Printf("Data Size: %s\n", formatBytes(status.DataSize))

	return nil
}

func handleStop(client *ControlClient) error {
	fmt.Print("Stopping AmorphDB service... ")
	err := client.Stop()
	if err != nil {
		fmt.Println("failed")
		return err
	}
	fmt.Println("success")
	return nil
}

func handleCompact(client *ControlClient) error {
	fmt.Print("Starting data compaction... ")
	err := client.Compact()
	if err != nil {
		fmt.Println("failed")
		return err
	}
	fmt.Println("success")
	return nil
}

func handleZonesList(client *ControlClient) error {
	// Placeholder for Step 10 (mesh networking) functionality
	fmt.Println("Zone management not yet implemented (requires Step 10: Mesh Networking)")
	fmt.Println("Available in future version for distributed mesh operations")
	return nil
}

func handleCreateMesh(client *ControlClient, meshName string) error {
	// Create mesh command using the new implementation
	meshCommand := NewMeshCommand(client)
	return meshCommand.CreateMesh(meshName)
}

func handleJoin(client *ControlClient, address string) error {
	joinCommand := NewJoinCommand(client)
	return joinCommand.JoinMesh(address)
}

func handleBridge(client *ControlClient, address string) error {
	bridgeCommand := NewBridgeCommand(client)
	return bridgeCommand.CreateBridge(address)
}

func handleDetach(client *ControlClient, meshName string) error {
	detachCommand := NewDetachCommand(client)
	return detachCommand.DetachFromMesh(meshName)
}

func handleInitPWA(appName, targetDir string) error {
	cmd := NewInitPWACommand(appName, targetDir)
	return cmd.Run()
}

func handleExtract(client *ControlClient, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("extract command requires a path")
	}

	path := args[0]
	var outputFile string

	// Parse optional -o flag
	if len(args) >= 3 && args[1] == "-o" {
		outputFile = args[2]
	} else if len(args) == 2 && args[1] != "-o" {
		return fmt.Errorf("invalid extract arguments. Usage: extract <path> [-o output_file]")
	} else if len(args) > 3 {
		return fmt.Errorf("too many arguments for extract command")
	}

	extractCommand := NewExtractCommand(client)
	return extractCommand.ExtractSubtree(path, outputFile)
}

func formatDuration(seconds int64) string {
	duration := time.Duration(seconds) * time.Second
	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60
	secs := int(duration.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, secs)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, secs)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	} else {
		return fmt.Sprintf("%ds", secs)
	}
}

func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

func showHelp() {
	fmt.Println("AmorphDB Control Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  %s <command>\n", os.Args[0])
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  status            Show service status and statistics")
	fmt.Println("  stop              Gracefully stop the AmorphDB service")
	fmt.Println("  compact           Trigger data defragmentation")
	fmt.Println("  zones list        List zone assignments (placeholder for Step 10)")
	fmt.Println("  create-mesh <name>  Create a new mesh with the given name")
	fmt.Println("  join <address>    Join an existing mesh at the given address")
	fmt.Println("  bridge <address>  Create a bridge to another mesh")
	fmt.Println("  detach [mesh]     Detach from mesh (or primary mesh if no name given)")
	fmt.Println("  extract <path> [-o file]  Extract subtree as MBL script")
	fmt.Println("  init-pwa <appname> <directory>  Scaffold a new PWA project")
	fmt.Println("  help              Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Printf("  %s status              # Show current service status\n", os.Args[0])
	fmt.Printf("  %s stop                # Stop the service gracefully\n", os.Args[0])
	fmt.Printf("  %s compact             # Compact and defragment data\n", os.Args[0])
	fmt.Printf("  %s create-mesh mynet   # Create a new mesh named 'mynet'\n", os.Args[0])
	fmt.Printf("  %s join 192.168.1.10   # Join mesh at address\n", os.Args[0])
	fmt.Printf("  %s bridge 10.0.0.5     # Bridge to another mesh\n", os.Args[0])
	fmt.Printf("  %s detach              # Leave current mesh\n", os.Args[0])
	fmt.Printf("  %s extract world.myapp -o backup.mbl  # Extract to file\n", os.Args[0])
	fmt.Printf("  %s extract world.myapp # Extract to stdout\n", os.Args[0])
	fmt.Printf("  %s init-pwa myapp ~/projects/myapp/  # Create starter PWA project\n", os.Args[0])
	fmt.Println()
	fmt.Println("Note: Most commands connect via local UNIX socket; init-pwa is local-only.")
	fmt.Println("The AmorphDB service (amorphd) must be running.")
	fmt.Println()
}