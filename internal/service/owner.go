package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/solifugus/amorphdb/internal/mesh"
	"github.com/solifugus/amorphdb/internal/security"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// ownerMarkerPath is the well-known location recording the node owner's identity
// (a CV-syllable string). Its presence is how the node knows it has been
// initialized with an owner.
var ownerMarkerPath = []string{"world", "node", "owner"}

// deviceSecretFile is the basename of the host-local device-secret file, stored
// under the storage directory with 0600 permissions. It is the "something you
// have" authentication factor and is deliberately never written into the mesh.
const deviceSecretFile = "device_secret"

// ownerAgentID maps a CV-syllable identity string to the numeric agent ID used
// as the home segment (world.agent.{id}) and write author. It mirrors the
// storage layer's identity hash so the two conventions agree; if they are ever
// unified, this is the single place to change.
func ownerAgentID(identity string) uint64 {
	hash := uint64(0)
	for i, c := range identity {
		hash = hash*31 + uint64(c) + uint64(i)
	}
	return hash
}

// OwnerIdentity returns the recorded owner's CV identity string, or ("", false)
// if the node has not been initialized with an owner.
func (s *Service) OwnerIdentity() (string, bool) {
	v, err := s.tree.Read(ownerMarkerPath)
	if err != nil || v.TypeTag == types.TypeNothing {
		return "", false
	}
	decoded, err := types.DeserializeValue(types.SerializedValue{TypeTag: v.TypeTag, Data: v.Data})
	if err != nil {
		return "", false
	}
	text, ok := decoded.(types.Text)
	if !ok || text.Value == "" {
		return "", false
	}
	return text.Value, true
}

// OwnerAgentID returns the owner's numeric agent ID and true when an owner has
// been established, else (0, false).
func (s *Service) OwnerAgentID() (uint64, bool) {
	identity, ok := s.OwnerIdentity()
	if !ok {
		return 0, false
	}
	return ownerAgentID(identity), true
}

// LookupAgentPublicKey returns the stored public key for an enrolled agent
// identity, reading world.agent.{ownerAgentID(identity)}.keys.public (hex of the
// big.Int bytes, as written by InitOwner). It returns (key, true) when a public
// key is present, else (nil, false). This is what the connection handshake uses
// to build a challenge the genuine agent can answer.
func (s *Service) LookupAgentPublicKey(identity string) (*big.Int, bool) {
	id := ownerAgentID(identity)
	path := []string{"world", "agent", fmt.Sprintf("%d", id), "keys", "public"}
	v, err := s.tree.Read(path)
	if err != nil || v.TypeTag == types.TypeNothing {
		return nil, false
	}
	decoded, err := types.DeserializeValue(types.SerializedValue{TypeTag: v.TypeTag, Data: v.Data})
	if err != nil {
		return nil, false
	}
	text, ok := decoded.(types.Text)
	if !ok || text.Value == "" {
		return nil, false
	}
	raw, err := hex.DecodeString(text.Value)
	if err != nil {
		return nil, false
	}
	return new(big.Int).SetBytes(raw), true
}

// InitOwner bootstraps the node owner: the genesis agent that owns this host.
// It is a one-time operation — it fails if an owner already exists. It generates
// a CV-syllable identity, ensures a host-local device secret, derives the owner's
// keypair from (passphrase, deviceSecret), stores the public key in the mesh and
// the encrypted private key beside it, and records the owner marker. Writes go
// directly to the tree (system author), bypassing the data-plane permission
// check, which is correct: the owner cannot yet authenticate to authorize its
// own creation. Returns the generated CV identity.
func (s *Service) InitOwner(passphrase string) (string, error) {
	if passphrase == "" {
		return "", fmt.Errorf("passphrase is required")
	}
	if existing, ok := s.OwnerIdentity(); ok {
		return "", fmt.Errorf("owner already initialized (%s)", existing)
	}

	identity, err := mesh.GenerateNodeIdentity()
	if err != nil {
		return "", fmt.Errorf("generate owner identity: %w", err)
	}

	deviceSecret, err := s.loadOrCreateDeviceSecret()
	if err != nil {
		return "", err
	}

	auth := security.NewAgentAuthenticator()
	agentIdentity, err := auth.CreateAgentIdentity(identity, passphrase, deviceSecret)
	if err != nil {
		return "", fmt.Errorf("derive owner keypair: %w", err)
	}

	// Encrypt the private key with the agent's derived key so it can live in the
	// mesh at rest without exposing the key material.
	enc := security.NewAgentEncryption(passphrase, deviceSecret)
	encryptedPrivate, err := enc.EncryptSecret(agentIdentity.KeyPair.Private.Bytes())
	if err != nil {
		return "", fmt.Errorf("encrypt owner private key: %w", err)
	}

	id := ownerAgentID(identity)
	home := []string{"world", "agent", fmt.Sprintf("%d", id)}
	writes := []struct {
		path  []string
		value types.Text
	}{
		{append(append([]string{}, home...), "keys", "public"), types.Text{Value: hex.EncodeToString(agentIdentity.PublicKey.Bytes())}},
		{append(append([]string{}, home...), "keys", "private"), types.Text{Value: hex.EncodeToString(encryptedPrivate)}},
		{ownerMarkerPath, types.Text{Value: identity}},
	}
	for _, w := range writes {
		val := storage.Value{TypeTag: w.value.TypeTag(), Data: w.value.Serialize()}
		if err := s.tree.Write(w.path, val, systemAuthor); err != nil {
			return "", fmt.Errorf("write %v: %w", w.path, err)
		}
	}

	return identity, nil
}

// loadOrCreateDeviceSecret returns the host-local device secret (hex-encoded),
// creating it with 0600 permissions on first use. The secret is never written
// into the mesh — it is the device-binding factor and stays on this host.
func (s *Service) loadOrCreateDeviceSecret() (string, error) {
	path := filepath.Join(s.storageDir, deviceSecretFile)
	if data, err := os.ReadFile(path); err == nil {
		return string(data), nil
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate device secret: %w", err)
	}
	secret := hex.EncodeToString(raw)
	if err := os.WriteFile(path, []byte(secret), 0600); err != nil {
		return "", fmt.Errorf("write device secret: %w", err)
	}
	return secret, nil
}
