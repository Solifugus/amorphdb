package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/solifugus/amorphdb/internal/security"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// Invites are the grant-gated primitive for enrolling additional mesh agents. An
// agent holding @grant on world.agent (the owner, by default, and anyone the
// owner delegates to) issues a one-time, time-limited token. A prospective agent
// redeems the token to register the PUBLIC key it generated on its own device —
// the private key never leaves that device. See auth-bootstrap-design (Stage 4)
// and amorphdb_design.md §Mobile Agent Authentication.

// invitesBasePath is the well-known subtree holding outstanding invites, keyed by
// a hash of the token so the raw token never rests in the mesh (world @read is
// open by default; only the unusable hash is visible).
var invitesBasePath = []string{"world", "node", "invites"}

// inviteTTL bounds how long an issued invite remains redeemable.
const inviteTTL = 24 * time.Hour

// hashInviteToken returns the hex-encoded SHA-256 of a raw token, used as its
// storage key. The raw token is the bearer secret; only its hash is stored.
func hashInviteToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// invitePath returns the subtree path for a given token hash.
func invitePath(tokenHash string, leaf ...string) []string {
	p := append(append([]string{}, invitesBasePath...), tokenHash)
	return append(p, leaf...)
}

// CanIssueInvites reports whether the given agent may issue enrollment invites.
// Authority is the @grant permission on world.agent: the owner holds it by
// default (granted at InitOwner) and may delegate it to other admins through the
// normal permission system, so "delegated org admin" falls out for free.
func (s *Service) CanIssueInvites(agentID uint64) bool {
	agent := &security.Agent{
		Identity: fmt.Sprintf("agent-%d", agentID),
		AgentID:  agentID,
		Tree:     s.tree,
	}
	return s.permEvaluator.CanGrant(agent, []string{"world", "agent"})
}

// CreateInvite issues a one-time, time-limited enrollment token on behalf of
// issuerAgentID, provided that agent is authorized (@grant on world.agent). It
// returns the raw token, which the caller delivers to the prospective agent; the
// mesh stores only the token's hash, its issuer, an unused flag, and an expiry.
func (s *Service) CreateInvite(issuerAgentID uint64) (string, error) {
	if !s.CanIssueInvites(issuerAgentID) {
		return "", fmt.Errorf("agent %d is not authorized to issue invites", issuerAgentID)
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate invite token: %w", err)
	}
	token := hex.EncodeToString(raw)
	tokenHash := hashInviteToken(token)

	expires := time.Now().Add(inviteTTL).UnixMicro()
	writes := []struct {
		leaf  string
		value types.Value
	}{
		{"issuer", types.Text{Value: fmt.Sprintf("%d", issuerAgentID)}},
		{"used", types.Boolean{Value: false}},
		{"expires", types.Number{Value: float64(expires)}},
	}
	for _, w := range writes {
		val := storage.Value{TypeTag: w.value.TypeTag(), Data: w.value.Serialize()}
		if err := s.tree.Write(invitePath(tokenHash, w.leaf), val, systemAuthor); err != nil {
			return "", fmt.Errorf("store invite %s: %w", w.leaf, err)
		}
	}

	return token, nil
}

// VerifyAndConsumeInvite validates a redeemed token and marks it used, returning
// the issuer's agent ID on success. It fails if the token is unknown, already
// used, or expired. Consumption is single-use: a successful call flips the used
// flag so the same token cannot be redeemed twice.
func (s *Service) VerifyAndConsumeInvite(token string) (uint64, error) {
	tokenHash := hashInviteToken(token)

	usedText, ok := s.readInviteLeaf(tokenHash, "used")
	if !ok {
		return 0, fmt.Errorf("unknown invite")
	}
	used, ok := usedText.(types.Boolean)
	if !ok {
		return 0, fmt.Errorf("malformed invite")
	}
	if used.Value {
		return 0, fmt.Errorf("invite already used")
	}

	expiresVal, ok := s.readInviteLeaf(tokenHash, "expires")
	if !ok {
		return 0, fmt.Errorf("malformed invite")
	}
	expires, ok := expiresVal.(types.Number)
	if !ok {
		return 0, fmt.Errorf("malformed invite")
	}
	if time.Now().UnixMicro() > int64(expires.Value) {
		return 0, fmt.Errorf("invite expired")
	}

	issuerVal, ok := s.readInviteLeaf(tokenHash, "issuer")
	if !ok {
		return 0, fmt.Errorf("malformed invite")
	}
	issuerText, ok := issuerVal.(types.Text)
	if !ok {
		return 0, fmt.Errorf("malformed invite")
	}
	var issuerAgentID uint64
	if _, err := fmt.Sscanf(issuerText.Value, "%d", &issuerAgentID); err != nil {
		return 0, fmt.Errorf("malformed invite issuer: %w", err)
	}

	// Single-use: flip the used flag before returning success so the token cannot
	// be redeemed again.
	usedTrue := types.Boolean{Value: true}
	val := storage.Value{TypeTag: usedTrue.TypeTag(), Data: usedTrue.Serialize()}
	if err := s.tree.Write(invitePath(tokenHash, "used"), val, systemAuthor); err != nil {
		return 0, fmt.Errorf("consume invite: %w", err)
	}

	return issuerAgentID, nil
}

// readInviteLeaf reads and deserializes a single invite leaf, returning
// (value, true) when present and (nil, false) otherwise.
func (s *Service) readInviteLeaf(tokenHash, leaf string) (interface{}, bool) {
	v, err := s.tree.Read(invitePath(tokenHash, leaf))
	if err != nil || v.TypeTag == types.TypeNothing {
		return nil, false
	}
	decoded, err := types.DeserializeValue(types.SerializedValue{TypeTag: v.TypeTag, Data: v.Data})
	if err != nil {
		return nil, false
	}
	return decoded, true
}
