package service

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/protocol"
	"github.com/solifugus/amorphdb/internal/security"
)

// registerOverConn runs a REGISTER round trip and returns the decoded result.
func registerOverConn(t *testing.T, svc *Service, identity string, publicKeyBytes []byte, token string) *protocol.RegisterResultMessage {
	t.Helper()
	conn := runConnection(t, svc)
	payload, err := protocol.EncodeRegisterMessage(&protocol.RegisterMessage{
		Identity:  identity,
		PublicKey: publicKeyBytes,
		Token:     token,
	})
	if err != nil {
		t.Fatalf("encode register: %v", err)
	}
	writeFramedMessage(t, conn, protocol.REGISTER, 1, payload)
	resp := readFramedMessage(t, conn)
	if resp.Type != protocol.REGISTER_RESULT {
		t.Fatalf("expected REGISTER_RESULT, got 0x%02x", resp.Type)
	}
	result, err := protocol.DecodeRegisterResultMessage(resp.Payload)
	if err != nil {
		t.Fatalf("decode register result: %v", err)
	}
	return result
}

// TestRegister_ThenLogin is the full enrollment loop for an additional user: the
// owner issues an invite, a brand-new client generates its own key pair and
// registers only the public key with the token, and the new agent then
// authenticates via the challenge-response handshake — proving a non-owner agent
// can be enrolled and log in without the daemon ever seeing its private key.
func TestRegister_ThenLogin(t *testing.T) {
	svc, ownerIdentity, _ := startOwnedServiceNoListen(t, "correct horse battery staple")
	ownerID := ownerAgentID(ownerIdentity)

	// Owner issues an invite.
	token, err := svc.CreateInvite(ownerID)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	// The prospective agent generates its own key pair locally — the daemon
	// never sees the passphrase, the device secret, or the private key.
	const newIdentity = "kset-va-lo-mi"
	authenticator := security.NewAgentAuthenticator()
	newAgent, err := authenticator.CreateAgentIdentity(newIdentity, "newbie-pass", "newbie-device-secret")
	if err != nil {
		t.Fatalf("generate new agent keys: %v", err)
	}

	// Register the public key with the invite token.
	result := registerOverConn(t, svc, newIdentity, newAgent.PublicKey.Bytes(), token)
	if !result.Success {
		t.Fatalf("registration should succeed: %s", result.Error)
	}
	wantID := ownerAgentID(newIdentity)
	if result.AgentID != wantID {
		t.Errorf("registered AgentID = %d, want %d", result.AgentID, wantID)
	}
	if wantID == ownerID {
		t.Fatal("new agent id collides with the owner id (test identity chosen poorly)")
	}

	// The public key must now be resolvable.
	if _, ok := svc.LookupAgentPublicKey(newIdentity); !ok {
		t.Fatal("registered public key should be looked up")
	}

	// The new agent authenticates via the Stage 3 handshake on a fresh
	// connection and is bound to its own identity.
	conn := runConnection(t, svc)
	initPayload, err := protocol.EncodeAuthInitMessage(&protocol.AuthInitMessage{Identity: newIdentity})
	if err != nil {
		t.Fatalf("encode auth init: %v", err)
	}
	writeFramedMessage(t, conn, protocol.AUTH_INIT, 1, initPayload)
	chalMsg := readFramedMessage(t, conn)
	if chalMsg.Type != protocol.AUTH_CHALLENGE {
		t.Fatalf("expected AUTH_CHALLENGE, got 0x%02x", chalMsg.Type)
	}
	respPayload := answerChallenge(t, chalMsg.Payload, newIdentity, newAgent.KeyPair)
	writeFramedMessage(t, conn, protocol.AUTH_RESPONSE, 2, respPayload)
	resultMsg := readFramedMessage(t, conn)
	authResult, err := protocol.DecodeAuthResultMessage(resultMsg.Payload)
	if err != nil {
		t.Fatalf("decode auth result: %v", err)
	}
	if !authResult.Success {
		t.Fatalf("newly-enrolled agent should authenticate: %s", authResult.Error)
	}
	if authResult.AgentID != wantID {
		t.Errorf("auth-bound AgentID = %d, want %d", authResult.AgentID, wantID)
	}
	if authResult.Identity != newIdentity {
		t.Errorf("auth-bound Identity = %q, want %q", authResult.Identity, newIdentity)
	}
}

// TestRegister_BadTokenRejected verifies registration with an unissued token is
// refused and no public key is stored.
func TestRegister_BadTokenRejected(t *testing.T) {
	svc, _, _ := startOwnedServiceNoListen(t, "correct horse battery staple")

	authenticator := security.NewAgentAuthenticator()
	agent, err := authenticator.CreateAgentIdentity("noname", "pass", "device")
	if err != nil {
		t.Fatalf("generate keys: %v", err)
	}

	result := registerOverConn(t, svc, "noname", agent.PublicKey.Bytes(), "not-a-real-token")
	if result.Success {
		t.Fatal("registration with a bogus token must fail")
	}
	if _, ok := svc.LookupAgentPublicKey("noname"); ok {
		t.Fatal("no public key should be stored after a failed registration")
	}
}

// TestRegister_TokenIsSingleUse verifies an invite token cannot enroll two
// agents: the second registration with the same token is refused.
func TestRegister_TokenIsSingleUse(t *testing.T) {
	svc, ownerIdentity, _ := startOwnedServiceNoListen(t, "correct horse battery staple")
	ownerID := ownerAgentID(ownerIdentity)

	token, err := svc.CreateInvite(ownerID)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	authenticator := security.NewAgentAuthenticator()
	first, err := authenticator.CreateAgentIdentity("first-agent", "p1", "d1")
	if err != nil {
		t.Fatalf("generate first keys: %v", err)
	}
	second, err := authenticator.CreateAgentIdentity("second-agent", "p2", "d2")
	if err != nil {
		t.Fatalf("generate second keys: %v", err)
	}

	if r := registerOverConn(t, svc, "first-agent", first.PublicKey.Bytes(), token); !r.Success {
		t.Fatalf("first registration should succeed: %s", r.Error)
	}
	if r := registerOverConn(t, svc, "second-agent", second.PublicKey.Bytes(), token); r.Success {
		t.Fatal("second registration with the same token must fail")
	}
}
