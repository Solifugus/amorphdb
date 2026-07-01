package service

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// expireInPast overwrites an invite's expiry leaf with a timestamp in the past,
// so VerifyAndConsumeInvite sees it as expired without the test having to wait.
func expireInPast(t *testing.T, svc *Service, tokenHash string) {
	t.Helper()
	past := types.Number{Value: 1} // 1 microsecond after the epoch — long gone
	val := storage.Value{TypeTag: past.TypeTag(), Data: past.Serialize()}
	if err := svc.tree.Write(invitePath(tokenHash, "expires"), val, systemAuthor); err != nil {
		t.Fatalf("overwrite expiry: %v", err)
	}
}

// TestInvite_OwnerCanIssue verifies the owner is authorized to issue invites
// (it holds @grant on world.agent, granted at InitOwner) and that a freshly
// issued token verifies exactly once.
func TestInvite_OwnerCanIssue(t *testing.T) {
	svc, identity, _ := startOwnedServiceNoListen(t, "correct horse battery staple")
	ownerID := ownerAgentID(identity)

	if !svc.CanIssueInvites(ownerID) {
		t.Fatal("owner should be authorized to issue invites")
	}

	token, err := svc.CreateInvite(ownerID)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	if token == "" {
		t.Fatal("CreateInvite returned an empty token")
	}

	issuer, err := svc.VerifyAndConsumeInvite(token)
	if err != nil {
		t.Fatalf("VerifyAndConsumeInvite: %v", err)
	}
	if issuer != ownerID {
		t.Errorf("invite issuer = %d, want owner %d", issuer, ownerID)
	}
}

// TestInvite_NonAuthorizedCannotIssue verifies the anonymous default agent
// (1000), which lacks @grant on world.agent, cannot issue invites.
func TestInvite_NonAuthorizedCannotIssue(t *testing.T) {
	svc, _, _ := startOwnedServiceNoListen(t, "correct horse battery staple")

	const anonymous uint64 = 1000
	if svc.CanIssueInvites(anonymous) {
		t.Fatal("anonymous agent must not be authorized to issue invites")
	}

	if _, err := svc.CreateInvite(anonymous); err == nil {
		t.Fatal("CreateInvite should fail for an unauthorized agent")
	}
}

// TestInvite_SingleUse verifies a token can be redeemed only once: the second
// redemption fails with the token already marked used.
func TestInvite_SingleUse(t *testing.T) {
	svc, identity, _ := startOwnedServiceNoListen(t, "correct horse battery staple")
	ownerID := ownerAgentID(identity)

	token, err := svc.CreateInvite(ownerID)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	if _, err := svc.VerifyAndConsumeInvite(token); err != nil {
		t.Fatalf("first redemption should succeed: %v", err)
	}
	if _, err := svc.VerifyAndConsumeInvite(token); err == nil {
		t.Fatal("second redemption of the same token must fail")
	}
}

// TestInvite_UnknownTokenRejected verifies a token that was never issued is
// rejected.
func TestInvite_UnknownTokenRejected(t *testing.T) {
	svc, _, _ := startOwnedServiceNoListen(t, "correct horse battery staple")

	if _, err := svc.VerifyAndConsumeInvite("deadbeefnotarealtoken"); err == nil {
		t.Fatal("an unissued token must be rejected")
	}
}

// TestInvite_ExpiredRejected verifies an invite past its expiry is rejected.
// It writes an already-expired invite record directly (the same leaves
// CreateInvite writes) rather than sleeping, so the test is fast and
// deterministic.
func TestInvite_ExpiredRejected(t *testing.T) {
	svc, identity, _ := startOwnedServiceNoListen(t, "correct horse battery staple")
	ownerID := ownerAgentID(identity)

	token, err := svc.CreateInvite(ownerID)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	// Overwrite the expiry with a time in the past.
	tokenHash := hashInviteToken(token)
	expireInPast(t, svc, tokenHash)

	if _, err := svc.VerifyAndConsumeInvite(token); err == nil {
		t.Fatal("an expired invite must be rejected")
	}
}
