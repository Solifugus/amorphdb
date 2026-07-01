package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/security"
	"github.com/solifugus/amorphdb/internal/service"
)

// TestEnrollThenLogin_ClientFlow exercises the amorph client enrollment path
// against a real daemon: an owner issues an invite, `runEnroll` generates a key
// pair locally and registers only the public key (persisting the device secret),
// and the client then authenticates by re-deriving the key pair from the stored
// device secret plus the passphrase — end to end through the actual CLI helpers.
func TestEnrollThenLogin_ClientFlow(t *testing.T) {
	// Credential files must land in an isolated HOME, and the passphrase prompt
	// is fed via the environment so the test is non-interactive.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AMORPH_PASSPHRASE", "enrollee-passphrase")

	cfg := service.DefaultConfig()
	cfg.StorageDir = t.TempDir()
	cfg.LocalSocketPath = filepath.Join(os.TempDir(), fmt.Sprintf("amorph-enroll-%d.sock", os.Getpid()))
	cfg.NetworkPort = 0

	svc, err := service.New(cfg)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	if _, err := svc.InitOwner("owner-passphrase"); err != nil {
		t.Fatalf("InitOwner: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { svc.Stop() })
	time.Sleep(100 * time.Millisecond)

	ownerID, ok := svc.OwnerAgentID()
	if !ok {
		t.Fatal("owner should be established")
	}
	token, err := svc.CreateInvite(ownerID)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	socket := svc.GetLocalSocketPath()
	const newIdentity = "test-enrollee"

	// Enroll: generates keys, registers the public key, persists the device secret.
	if err := runEnroll(socket, newIdentity, token); err != nil {
		t.Fatalf("runEnroll: %v", err)
	}

	// The device secret must have been persisted with 0600 permissions.
	secretPath, err := deviceSecretPath(newIdentity)
	if err != nil {
		t.Fatalf("deviceSecretPath: %v", err)
	}
	info, err := os.Stat(secretPath)
	if err != nil {
		t.Fatalf("device secret should exist after enroll: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("device secret perms = %o, want 0600", perm)
	}

	// Log in: re-derive the key pair from the stored device secret + passphrase
	// and authenticate. (runLogin itself opens the blocking REPL, so we drive its
	// pieces directly here.)
	deviceSecret, err := loadDeviceSecret(newIdentity)
	if err != nil {
		t.Fatalf("loadDeviceSecret: %v", err)
	}
	authenticator := security.NewAgentAuthenticator()
	agentIdentity, err := authenticator.CreateAgentIdentity(newIdentity, "enrollee-passphrase", deviceSecret)
	if err != nil {
		t.Fatalf("re-derive key pair: %v", err)
	}

	client, err := NewProtocolClient(socket)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	agentID, boundIdentity, err := client.Authenticate(newIdentity, agentIdentity.KeyPair)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if boundIdentity != newIdentity {
		t.Errorf("bound identity = %q, want %q", boundIdentity, newIdentity)
	}
	if agentID == ownerID {
		t.Error("enrolled agent must not share the owner's id")
	}

	// WHOAMI on the same connection must now report the enrolled identity.
	id, name, err := client.WhoAmI()
	if err != nil {
		t.Fatalf("WhoAmI: %v", err)
	}
	if id != agentID || name != newIdentity {
		t.Errorf("WHOAMI = (%d, %q), want (%d, %q)", id, name, agentID, newIdentity)
	}
}

// TestEnroll_BadTokenNoCredentials verifies a failed enrollment (bad token)
// leaves no device secret behind on disk.
func TestEnroll_BadTokenNoCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AMORPH_PASSPHRASE", "whatever")

	cfg := service.DefaultConfig()
	cfg.StorageDir = t.TempDir()
	cfg.LocalSocketPath = filepath.Join(os.TempDir(), fmt.Sprintf("amorph-enroll-bad-%d.sock", os.Getpid()))
	cfg.NetworkPort = 0

	svc, err := service.New(cfg)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	if _, err := svc.InitOwner("owner-passphrase"); err != nil {
		t.Fatalf("InitOwner: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { svc.Stop() })
	time.Sleep(100 * time.Millisecond)

	if err := runEnroll(svc.GetLocalSocketPath(), "doomed", "bogus-token"); err == nil {
		t.Fatal("enroll with a bogus token should fail")
	}

	if path, err := deviceSecretPath("doomed"); err == nil {
		if _, statErr := os.Stat(path); statErr == nil {
			t.Error("no device secret should be persisted after a failed enroll")
		}
	}
}
