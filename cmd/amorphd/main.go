// Package main implements the AmorphDB service daemon
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/solifugus/amorphdb/internal/service"
)

func main() {
	// Command line flags
	var (
		storageDir   = flag.String("data", "", "Data storage directory (default: ~/.amorph/data)")
		socketPath   = flag.String("socket", "", "UNIX socket path (default: ~/.amorph/socket)")
		port         = flag.Int("port", 5000, "Network port for remote connections")
		nodeIdentity = flag.String("node", "", "Node identity (default: auto-generated)")
		help         = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	// Show help
	if *help {
		showHelp()
		return
	}

	// Get default configuration
	config := service.DefaultConfig()

	// Override with command line flags
	if *storageDir != "" {
		config.StorageDir = *storageDir
	}
	if *socketPath != "" {
		config.LocalSocketPath = *socketPath
	}
	if *port != 5000 {
		config.NetworkPort = *port
	}
	if *nodeIdentity != "" {
		config.NodeIdentity = *nodeIdentity
	}

	// Ensure storage and socket directories exist
	if err := os.MkdirAll(config.StorageDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create storage directory: %v\n", err)
		os.Exit(1)
	}

	socketDir := filepath.Dir(config.LocalSocketPath)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create socket directory: %v\n", err)
		os.Exit(1)
	}

	// Create service
	svc, err := service.New(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create service: %v\n", err)
		os.Exit(1)
	}

	// Start service
	if err := svc.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start service: %v\n", err)
		os.Exit(1)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	fmt.Println("AmorphDB service running. Press Ctrl+C to stop.")
	<-sigChan

	// Graceful shutdown
	fmt.Println("\nReceived shutdown signal, stopping service...")
	if err := svc.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Error stopping service: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Service stopped successfully.")
}

func showHelp() {
	fmt.Println("AmorphDB Service Daemon")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  %s [options]\n", os.Args[0])
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -data <dir>      Data storage directory (default: ~/.amorph/data)")
	fmt.Println("  -socket <path>   UNIX socket path (default: ~/.amorph/socket)")
	fmt.Println("  -port <number>   Network port for remote connections (default: 5000)")
	fmt.Println("  -node <id>       Node identity (default: auto-generated)")
	fmt.Println("  -help            Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Printf("  %s                           # Start with default configuration\n", os.Args[0])
	fmt.Printf("  %s -port 6000               # Use custom port\n", os.Args[0])
	fmt.Printf("  %s -data /opt/amorphdb      # Use custom storage directory\n", os.Args[0])
	fmt.Println()
	fmt.Println("The service accepts connections on both:")
	fmt.Println("  - Local UNIX socket (for amorph client and amorphctl)")
	fmt.Println("  - Network TCP socket (for remote clients and mesh networking)")
	fmt.Println()
}