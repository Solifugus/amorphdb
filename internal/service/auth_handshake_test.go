package service

import (
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/solifugus/amorphdb/internal/protocol"
	"github.com/solifugus/amorphdb/internal/security"
)

// startOwnedServiceNoListen builds a service with an initialized owner but does
// NOT start its network/socket listeners. The connection handshake is exercised
// directly over an in-memory net.Pipe, so no real listener is needed. It returns
// the service, the owner's CV identity, and the host-local device secret (read
// from the storage dir) — the "something you have" factor a genuine remote
// client would have been provisioned with at enrollment.
func startOwnedServiceNoListen(t *testing.T, passphrase string) (*Service, string, string) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.StorageDir = t.TempDir()
	cfg.NetworkPort = 0

	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	identity, err := svc.InitOwner(passphrase)
	if err != nil {
		t.Fatalf("InitOwner: %v", err)
	}
	// The service's context must be live for Connection.Start() to loop.
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { svc.Stop() })

	secretBytes, err := os.ReadFile(filepath.Join(svc.storageDir, deviceSecretFile))
	if err != nil {
		t.Fatalf("read device secret: %v", err)
	}
	return svc, identity, string(secretBytes)
}

// writeFramedMessage encodes and writes a full protocol message to conn.
func writeFramedMessage(t *testing.T, conn net.Conn, msgType uint8, seq uint32, payload []byte) {
	t.Helper()
	data, err := protocol.EncodeMessage(protocol.CreateMessage(msgType, seq, payload))
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}
	if _, err := conn.Write(data); err != nil {
		t.Fatalf("write message: %v", err)
	}
}

// readFramedMessage reads one complete protocol message (header + payload +
// checksum) from conn, using the same 10-byte header framing as the wire format.
func readFramedMessage(t *testing.T, conn net.Conn) *protocol.Message {
	t.Helper()
	header := make([]byte, 10)
	if _, err := io.ReadFull(conn, header); err != nil {
		t.Fatalf("read header: %v", err)
	}
	payloadLen := binary.BigEndian.Uint32(header[6:10])
	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(conn, payload); err != nil {
		t.Fatalf("read payload: %v", err)
	}
	checksum := make([]byte, 4)
	if _, err := io.ReadFull(conn, checksum); err != nil {
		t.Fatalf("read checksum: %v", err)
	}
	full := append(append(append([]byte{}, header...), payload...), checksum...)
	msg, err := protocol.DecodeMessage(full)
	if err != nil {
		t.Fatalf("decode message: %v", err)
	}
	return msg
}

// answerChallenge deserializes an AUTH_CHALLENGE payload, decrypts it with the
// given key pair, and returns the serialized AUTH_RESPONSE payload — exactly
// what a genuine client holding the private key would produce.
func answerChallenge(t *testing.T, challengePayload []byte, identity string, keyPair *security.KeyPair) []byte {
	t.Helper()
	chalWire, err := protocol.DecodeAuthChallengeMessage(challengePayload)
	if err != nil {
		t.Fatalf("decode auth challenge wrapper: %v", err)
	}
	chal, err := security.DeserializeAuthChallenge(chalWire.Challenge)
	if err != nil {
		t.Fatalf("deserialize auth challenge: %v", err)
	}
	authenticator := security.NewAgentAuthenticator()
	resp, err := authenticator.RespondToChallenge(&security.AuthenticationChallenge{
		ChallengeID:        chal.ChallengeID,
		EncryptedData:      chal.EncryptedData,
		EphemeralPublicKey: chal.EphemeralPublicKey,
	}, keyPair)
	if err != nil {
		t.Fatalf("respond to challenge: %v", err)
	}
	respWire := security.SerializeAuthResponse(&security.AuthResponseMessage{
		AgentIdentity: identity,
		ChallengeID:   resp.ChallengeID,
		DecryptedData: resp.DecryptedData,
	})
	out, err := protocol.EncodeAuthResponseMessage(&protocol.AuthResponseMessage{Response: respWire})
	if err != nil {
		t.Fatalf("encode auth response wrapper: %v", err)
	}
	return out
}

// runConnection wires a Connection to the server end of a pipe and starts its
// read loop, returning the client end for the test to drive.
func runConnection(t *testing.T, svc *Service) net.Conn {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	c := NewConnection("test-net", serverConn, false /* isLocal */, svc)
	go c.Start()
	t.Cleanup(func() { clientConn.Close() })
	return clientConn
}

// TestAuthHandshake_GenuineAgentIsBound verifies the full challenge-response
// handshake over the wire: a network connection starts anonymous (1000), and
// after proving possession of the owner's private key it is bound to the owner's
// identity — WHOAMI then reports the owner, confirming the upgrade stuck.
func TestAuthHandshake_GenuineAgentIsBound(t *testing.T) {
	passphrase := "correct horse battery staple"
	svc, identity, deviceSecret := startOwnedServiceNoListen(t, passphrase)

	// Reconstruct the owner's key pair from (passphrase, deviceSecret), as a
	// provisioned client would.
	authenticator := security.NewAgentAuthenticator()
	agentIdentity, err := authenticator.CreateAgentIdentity(identity, passphrase, deviceSecret)
	if err != nil {
		t.Fatalf("reconstruct agent identity: %v", err)
	}

	conn := runConnection(t, svc)

	// A network connection is anonymous before authenticating.
	writeFramedMessage(t, conn, protocol.WHOAMI, 1, nil)
	pre := readFramedMessage(t, conn)
	who, err := protocol.DecodeWhoAmIResponseMessage(pre.Payload)
	if err != nil {
		t.Fatalf("decode pre-auth whoami: %v", err)
	}
	if who.AgentID != 1000 {
		t.Fatalf("network connection should start anonymous (1000), got %d", who.AgentID)
	}

	// AUTH_INIT -> AUTH_CHALLENGE.
	initPayload, err := protocol.EncodeAuthInitMessage(&protocol.AuthInitMessage{Identity: identity})
	if err != nil {
		t.Fatalf("encode auth init: %v", err)
	}
	writeFramedMessage(t, conn, protocol.AUTH_INIT, 2, initPayload)
	chalMsg := readFramedMessage(t, conn)
	if chalMsg.Type != protocol.AUTH_CHALLENGE {
		t.Fatalf("expected AUTH_CHALLENGE, got 0x%02x", chalMsg.Type)
	}

	// AUTH_RESPONSE -> AUTH_RESULT.
	respPayload := answerChallenge(t, chalMsg.Payload, identity, agentIdentity.KeyPair)
	writeFramedMessage(t, conn, protocol.AUTH_RESPONSE, 3, respPayload)
	resultMsg := readFramedMessage(t, conn)
	if resultMsg.Type != protocol.AUTH_RESULT {
		t.Fatalf("expected AUTH_RESULT, got 0x%02x", resultMsg.Type)
	}
	result, err := protocol.DecodeAuthResultMessage(resultMsg.Payload)
	if err != nil {
		t.Fatalf("decode auth result: %v", err)
	}
	if !result.Success {
		t.Fatalf("genuine agent should authenticate: %s", result.Error)
	}
	wantID := ownerAgentID(identity)
	if result.AgentID != wantID {
		t.Errorf("AUTH_RESULT AgentID = %d, want owner %d", result.AgentID, wantID)
	}
	if result.Identity != identity {
		t.Errorf("AUTH_RESULT Identity = %q, want %q", result.Identity, identity)
	}

	// The upgrade must persist on the connection: WHOAMI now reports the owner.
	writeFramedMessage(t, conn, protocol.WHOAMI, 4, nil)
	post := readFramedMessage(t, conn)
	postWho, err := protocol.DecodeWhoAmIResponseMessage(post.Payload)
	if err != nil {
		t.Fatalf("decode post-auth whoami: %v", err)
	}
	if postWho.AgentID != wantID {
		t.Errorf("post-auth WHOAMI AgentID = %d, want %d", postWho.AgentID, wantID)
	}
	if postWho.Identity != identity {
		t.Errorf("post-auth WHOAMI Identity = %q, want %q", postWho.Identity, identity)
	}
}

// TestAuthHandshake_ImposterRejected verifies that a client who does NOT hold
// the private key cannot authenticate. Without the private key the challenge
// cannot even be decrypted (the CDH property from Stage 2), so the best an
// attacker can do is forge a response with a guessed plaintext — which the
// server must reject, leaving the connection anonymous.
func TestAuthHandshake_ImposterRejected(t *testing.T) {
	passphrase := "correct horse battery staple"
	svc, identity, _ := startOwnedServiceNoListen(t, passphrase)

	conn := runConnection(t, svc)

	initPayload, err := protocol.EncodeAuthInitMessage(&protocol.AuthInitMessage{Identity: identity})
	if err != nil {
		t.Fatalf("encode auth init: %v", err)
	}
	writeFramedMessage(t, conn, protocol.AUTH_INIT, 1, initPayload)
	chalMsg := readFramedMessage(t, conn)
	if chalMsg.Type != protocol.AUTH_CHALLENGE {
		t.Fatalf("expected AUTH_CHALLENGE, got 0x%02x", chalMsg.Type)
	}

	// Forge a response: echo the challenge ID (public, on the wire) but supply a
	// guessed 32-byte plaintext, since the real challenge cannot be recovered
	// without the private key.
	chalWire, err := protocol.DecodeAuthChallengeMessage(chalMsg.Payload)
	if err != nil {
		t.Fatalf("decode challenge wrapper: %v", err)
	}
	chal, err := security.DeserializeAuthChallenge(chalWire.Challenge)
	if err != nil {
		t.Fatalf("deserialize challenge: %v", err)
	}
	forgedWire := security.SerializeAuthResponse(&security.AuthResponseMessage{
		AgentIdentity: identity,
		ChallengeID:   chal.ChallengeID,
		DecryptedData: make([]byte, 32), // wrong plaintext
	})
	respPayload, err := protocol.EncodeAuthResponseMessage(&protocol.AuthResponseMessage{Response: forgedWire})
	if err != nil {
		t.Fatalf("encode forged response: %v", err)
	}
	writeFramedMessage(t, conn, protocol.AUTH_RESPONSE, 2, respPayload)
	resultMsg := readFramedMessage(t, conn)
	result, err := protocol.DecodeAuthResultMessage(resultMsg.Payload)
	if err != nil {
		t.Fatalf("decode auth result: %v", err)
	}
	if result.Success {
		t.Fatal("imposter without the private key must not authenticate")
	}

	// The connection must remain anonymous after a failed attempt.
	writeFramedMessage(t, conn, protocol.WHOAMI, 3, nil)
	post := readFramedMessage(t, conn)
	who, err := protocol.DecodeWhoAmIResponseMessage(post.Payload)
	if err != nil {
		t.Fatalf("decode post-fail whoami: %v", err)
	}
	if who.AgentID != 1000 {
		t.Errorf("connection should stay anonymous after failed auth, got %d", who.AgentID)
	}
}

// TestAuthHandshake_UnknownIdentityRefused verifies that AUTH_INIT for an
// identity with no stored public key is refused outright (no challenge issued).
func TestAuthHandshake_UnknownIdentityRefused(t *testing.T) {
	svc, _, _ := startOwnedServiceNoListen(t, "correct horse battery staple")

	conn := runConnection(t, svc)

	initPayload, err := protocol.EncodeAuthInitMessage(&protocol.AuthInitMessage{Identity: "nobody-here"})
	if err != nil {
		t.Fatalf("encode auth init: %v", err)
	}
	writeFramedMessage(t, conn, protocol.AUTH_INIT, 1, initPayload)
	resp := readFramedMessage(t, conn)
	if resp.Type != protocol.ERROR {
		t.Fatalf("expected ERROR for unknown identity, got 0x%02x", resp.Type)
	}
	errMsg, err := protocol.DecodeErrorMessage(resp.Payload)
	if err != nil {
		t.Fatalf("decode error message: %v", err)
	}
	if errMsg.Code != 401 {
		t.Errorf("expected 401 for unknown identity, got %d", errMsg.Code)
	}
}
