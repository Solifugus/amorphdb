// Package main implements the AmorphDB REPL (Read-Eval-Print Loop)
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	// Enrollment/login are subcommands (they take their own flags) rather than
	// top-level flags, so dispatch them before the default flag parsing.
	if len(os.Args) >= 2 && (os.Args[1] == "enroll" || os.Args[1] == "login") {
		if err := runAuthSubcommand(os.Args[1], os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "%s failed: %v\n", os.Args[1], err)
			os.Exit(1)
		}
		return
	}

	// Command line flags
	var (
		node     = flag.String("node", "", "Connect to remote AmorphDB service (host:port)")
		identity = flag.String("identity", "", "Agent identity for authentication")
		run      = flag.String("run", "", "Execute MBL script file then exit")
		help     = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	// Show help
	if *help {
		showHelp()
		return
	}

	// Determine connection mode
	var connectionAddress string
	if *node != "" {
		// Remote mode: connect via TCP
		connectionAddress = *node
	} else {
		// Local mode: connect via UNIX socket
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get user home directory: %v\n", err)
			os.Exit(1)
		}
		connectionAddress = filepath.Join(homeDir, ".amorph", "socket")
	}

	// Handle script mode
	if *run != "" {
		err := executeScript(*run, connectionAddress, *identity)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Script execution failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Create REPL instance
	repl, err := NewREPL(connectionAddress, *identity)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing AmorphDB client: %v\n", err)
		os.Exit(1)
	}

	// Ensure proper cleanup on exit
	defer repl.Close()

	// Start interactive REPL
	repl.Start()
}

// resolveAddress returns the TCP address when node is set, otherwise the local
// UNIX socket path.
func resolveAddress(node string) (string, error) {
	if node != "" {
		return node, nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".amorph", "socket"), nil
}

// runAuthSubcommand handles the `enroll` and `login` subcommands, each of which
// takes -node, -identity, and (for enroll) -token.
func runAuthSubcommand(command string, args []string) error {
	fs := flag.NewFlagSet(command, flag.ExitOnError)
	node := fs.String("node", "", "Connect to remote AmorphDB service (host:port)")
	identity := fs.String("identity", "", "Agent identity")
	token := fs.String("token", "", "One-time enrollment invite token (enroll only)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	address, err := resolveAddress(*node)
	if err != nil {
		return err
	}

	switch command {
	case "enroll":
		return runEnroll(address, *identity, *token)
	case "login":
		return runLogin(address, *identity)
	default:
		return fmt.Errorf("unknown subcommand %q", command)
	}
}

func showHelp() {
	fmt.Println("AmorphDB Client - MBL Interactive Shell")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  %s [options]\n", os.Args[0])
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -node <host:port>    Connect to remote AmorphDB service")
	fmt.Println("  -identity <agent>    Agent identity for authentication")
	fmt.Println("  -run <script.mbl>    Execute MBL script file then exit")
	fmt.Println("  -help                Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Printf("  %s                                    # Local REPL via UNIX socket\n", os.Args[0])
	fmt.Printf("  %s -node localhost:5000              # Remote REPL via TCP\n", os.Args[0])
	fmt.Printf("  %s -identity kalevo                  # Local REPL as specific agent\n", os.Args[0])
	fmt.Printf("  %s -run script.mbl                   # Execute script locally\n", os.Args[0])
	fmt.Printf("  %s -node host:5000 -run script.mbl   # Execute script remotely\n", os.Args[0])
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Printf("  %s enroll -node <host:port> -identity <name> -token <token>\n", os.Args[0])
	fmt.Println("                    Enroll as a new agent using an invite token")
	fmt.Printf("  %s login -node <host:port> -identity <name>\n", os.Args[0])
	fmt.Println("                    Authenticate as an enrolled agent, then open the REPL")
	fmt.Println()
	fmt.Println("Interactive Commands:")
	fmt.Println("  exit, quit, :q    Exit the REPL")
	fmt.Println("  help, ?           Show REPL help")
	fmt.Println()
}

func executeScript(scriptPath, address, identity string) error {
	// Read script file
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to read script: %w", err)
	}

	// Create client connection
	client, err := NewProtocolClient(address)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	// Create REPL for script execution
	repl, err := NewREPLWithClient(client, identity)
	if err != nil {
		return fmt.Errorf("failed to create REPL: %w", err)
	}
	defer repl.Close()

	// Execute script content
	result := repl.execute(string(content))
	if result != "" {
		fmt.Println(result)
	}

	return nil
}
