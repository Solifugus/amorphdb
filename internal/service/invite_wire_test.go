package service

import (
	"net"
	"testing"

	"github.com/solifugus/amorphdb/internal/protocol"
)

// runLocalConnection wires a Connection to the server end of a pipe with
// isLocal=true (so it auto-adopts the owner) and starts its read loop.
func runLocalConnection(t *testing.T, svc *Service) net.Conn {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	c := NewConnection("test-local", serverConn, true /* isLocal */, svc)
	go c.Start()
	t.Cleanup(func() { clientConn.Close() })
	return clientConn
}

// inviteCreateOverConn runs an INVITE_CREATE round trip and returns the result.
func inviteCreateOverConn(t *testing.T, conn net.Conn) *protocol.InviteCreateResultMessage {
	t.Helper()
	writeFramedMessage(t, conn, protocol.INVITE_CREATE, 1, nil)
	resp := readFramedMessage(t, conn)
	if resp.Type != protocol.INVITE_CREATE_RESULT {
		t.Fatalf("expected INVITE_CREATE_RESULT, got 0x%02x", resp.Type)
	}
	result, err := protocol.DecodeInviteCreateResultMessage(resp.Payload)
	if err != nil {
		t.Fatalf("decode invite result: %v", err)
	}
	return result
}

// TestInviteCreate_LocalOwnerMints verifies that the owner, over the local
// socket (auto-authed), can mint an invite via INVITE_CREATE and that the
// returned token is redeemable.
func TestInviteCreate_LocalOwnerMints(t *testing.T) {
	svc, _, _ := startOwnedServiceNoListen(t, "correct horse battery staple")

	conn := runLocalConnection(t, svc)
	result := inviteCreateOverConn(t, conn)
	if !result.Success {
		t.Fatalf("owner should be able to mint an invite: %s", result.Error)
	}
	if result.Token == "" {
		t.Fatal("a successful INVITE_CREATE must return a token")
	}
	if _, err := svc.VerifyAndConsumeInvite(result.Token); err != nil {
		t.Fatalf("minted token should be redeemable: %v", err)
	}
}

// TestInviteCreate_AnonymousRefused verifies that an anonymous network
// connection (the default agent, no @grant) cannot mint invites.
func TestInviteCreate_AnonymousRefused(t *testing.T) {
	svc, _, _ := startOwnedServiceNoListen(t, "correct horse battery staple")

	conn := runConnection(t, svc) // isLocal=false -> anonymous agent 1000
	result := inviteCreateOverConn(t, conn)
	if result.Success {
		t.Fatal("an anonymous connection must not be able to mint invites")
	}
}
