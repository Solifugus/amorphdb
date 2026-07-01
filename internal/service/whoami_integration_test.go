package service

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/protocol"
)

// startOwnedService builds a service with an initialized owner, on an isolated
// temp storage dir and a short unique socket path, and starts it. It returns the
// service and the owner's CV identity.
func startOwnedService(t *testing.T, passphrase string) (*Service, string) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.StorageDir = t.TempDir()
	// Keep the socket path short: AF_UNIX paths are capped at ~108 bytes, and
	// t.TempDir() under the CI sandbox can already be long.
	cfg.LocalSocketPath = filepath.Join(os.TempDir(), fmt.Sprintf("amorph-whoami-%d.sock", os.Getpid()))
	cfg.NetworkPort = 0

	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	identity, err := svc.InitOwner(passphrase)
	if err != nil {
		t.Fatalf("InitOwner: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { svc.Stop() })
	time.Sleep(100 * time.Millisecond)
	return svc, identity
}

// whoAmIOverSocket dials the given unix socket and performs a WHOAMI round trip.
func whoAmIOverSocket(t *testing.T, socketPath string) *protocol.WhoAmIResponseMessage {
	t.Helper()
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	req := protocol.CreateMessage(protocol.WHOAMI, 1, nil)
	data, err := protocol.EncodeMessage(req)
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}
	if _, err := conn.Write(data); err != nil {
		t.Fatalf("write request: %v", err)
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	respMsg, err := protocol.DecodeMessage(buf[:n])
	if err != nil {
		t.Fatalf("decode message: %v", err)
	}
	if respMsg.Type != protocol.WHOAMI_RESPONSE {
		t.Fatalf("expected WHOAMI_RESPONSE, got 0x%02x", respMsg.Type)
	}
	who, err := protocol.DecodeWhoAmIResponseMessage(respMsg.Payload)
	if err != nil {
		t.Fatalf("decode whoami: %v", err)
	}
	return who
}

// TestWhoAmI_LocalConnectionIsOwner verifies that a local (unix socket)
// connection is auto-authenticated as the node owner, and that WHOAMI reports
// the owner's numeric ID and CV identity — this is what lets the REPL resolve
// my.* under the owner's home.
func TestWhoAmI_LocalConnectionIsOwner(t *testing.T) {
	svc, identity := startOwnedService(t, "correct horse battery staple")

	who := whoAmIOverSocket(t, svc.GetLocalSocketPath())

	wantID := ownerAgentID(identity)
	if who.AgentID != wantID {
		t.Errorf("WHOAMI AgentID = %d, want owner %d", who.AgentID, wantID)
	}
	if who.Identity != identity {
		t.Errorf("WHOAMI Identity = %q, want %q", who.Identity, identity)
	}
	if who.AgentID == 1000 {
		t.Error("local connection still reports the default anonymous agent (1000)")
	}
}
