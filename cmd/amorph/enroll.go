package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/solifugus/amorphdb/internal/security"
)

// Agent credentials live under ~/.amorph/agents/{identity}/. The device secret
// (the "something you have" factor) is stored there with 0600 permissions and
// never leaves this machine; the passphrase (the "something you know" factor) is
// never stored — it is prompted each login and combined with the device secret
// to re-derive the key pair. See amorphdb_design.md §Mobile Agent Authentication.

// agentCredDir returns the per-identity credential directory.
func agentCredDir(identity string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".amorph", "agents", identity), nil
}

// deviceSecretPath returns the device-secret file path for an identity.
func deviceSecretPath(identity string) (string, error) {
	dir, err := agentCredDir(identity)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "device_secret"), nil
}

// generateDeviceSecret returns a fresh random 32-byte hex device secret.
func generateDeviceSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate device secret: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// saveDeviceSecret persists a device secret for an identity with 0600 perms.
func saveDeviceSecret(identity, secret string) error {
	dir, err := agentCredDir(identity)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create credential dir: %w", err)
	}
	path := filepath.Join(dir, "device_secret")
	if err := os.WriteFile(path, []byte(secret), 0600); err != nil {
		return fmt.Errorf("write device secret: %w", err)
	}
	return nil
}

// loadDeviceSecret reads a previously saved device secret for an identity.
func loadDeviceSecret(identity string) (string, error) {
	path, err := deviceSecretPath(identity)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("no credentials for %q on this device (enroll first): %w", identity, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// promptPassphrase reads a passphrase, preferring the AMORPH_PASSPHRASE
// environment variable (for scripting and tests) and otherwise reading a line
// from stdin. The input is not masked (no terminal dependency yet).
func promptPassphrase(prompt string) (string, error) {
	if p := os.Getenv("AMORPH_PASSPHRASE"); p != "" {
		return p, nil
	}
	fmt.Fprintf(os.Stderr, "%s: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("read passphrase: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// runEnroll enrolls this machine as a new agent: it generates a key pair from a
// freshly-created device secret plus a prompted passphrase, registers only the
// PUBLIC key with the daemon using the invite token, and — only on success —
// persists the device secret locally for future logins. The private key and
// passphrase never leave this process.
func runEnroll(address, identity, token string) error {
	if identity == "" {
		return fmt.Errorf("enroll requires -identity")
	}
	if token == "" {
		return fmt.Errorf("enroll requires -token")
	}

	passphrase, err := promptPassphrase("Choose a passphrase for " + identity)
	if err != nil {
		return err
	}
	if passphrase == "" {
		return fmt.Errorf("passphrase must not be empty")
	}

	deviceSecret, err := generateDeviceSecret()
	if err != nil {
		return err
	}

	authenticator := security.NewAgentAuthenticator()
	agentIdentity, err := authenticator.CreateAgentIdentity(identity, passphrase, deviceSecret)
	if err != nil {
		return fmt.Errorf("derive key pair: %w", err)
	}

	client, err := NewProtocolClient(address)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	agentID, err := client.Register(identity, agentIdentity.PublicKey.Bytes(), token)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	// Persist the device secret only after the daemon accepted the registration.
	if err := saveDeviceSecret(identity, deviceSecret); err != nil {
		return err
	}

	fmt.Printf("Enrolled as %s (agent %d).\n", identity, agentID)
	fmt.Printf("Log in with:\n  amorph login -node %s -identity %s\n", address, identity)
	return nil
}

// runLogin authenticates as an already-enrolled agent (re-deriving the key pair
// from the stored device secret plus a prompted passphrase) and then starts the
// REPL on the authenticated connection, so `my.*` resolves under that identity.
func runLogin(address, identity string) error {
	if identity == "" {
		return fmt.Errorf("login requires -identity")
	}

	deviceSecret, err := loadDeviceSecret(identity)
	if err != nil {
		return err
	}
	passphrase, err := promptPassphrase("Passphrase for " + identity)
	if err != nil {
		return err
	}

	authenticator := security.NewAgentAuthenticator()
	agentIdentity, err := authenticator.CreateAgentIdentity(identity, passphrase, deviceSecret)
	if err != nil {
		return fmt.Errorf("derive key pair: %w", err)
	}

	client, err := NewProtocolClient(address)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	agentID, boundIdentity, err := client.Authenticate(identity, agentIdentity.KeyPair)
	if err != nil {
		client.Close()
		return err
	}
	fmt.Printf("Authenticated as %s (agent %d).\n", boundIdentity, agentID)

	repl, err := NewREPLWithClient(client, "")
	if err != nil {
		client.Close()
		return fmt.Errorf("start REPL: %w", err)
	}
	defer repl.Close()
	repl.Start()
	return nil
}
