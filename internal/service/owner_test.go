package service

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/solifugus/amorphdb/internal/types"
)

func readText(t *testing.T, svc *Service, path []string) (string, bool) {
	t.Helper()
	v, err := svc.tree.Read(path)
	if err != nil || v.TypeTag == types.TypeNothing {
		return "", false
	}
	decoded, err := types.DeserializeValue(types.SerializedValue{TypeTag: v.TypeTag, Data: v.Data})
	if err != nil {
		return "", false
	}
	text, ok := decoded.(types.Text)
	if !ok {
		return "", false
	}
	return text.Value, true
}

func TestInitOwner_CreatesOwnerKeysAndDeviceSecret(t *testing.T) {
	svc := newSeedTestService(t)

	if _, ok := svc.OwnerIdentity(); ok {
		t.Fatal("expected no owner before init")
	}

	identity, err := svc.InitOwner("correct horse battery staple")
	if err != nil {
		t.Fatalf("InitOwner: %v", err)
	}
	if identity == "" {
		t.Fatal("expected a non-empty CV identity")
	}

	// Owner marker + numeric ID resolve.
	gotIdent, ok := svc.OwnerIdentity()
	if !ok || gotIdent != identity {
		t.Fatalf("OwnerIdentity = %q,%v; want %q,true", gotIdent, ok, identity)
	}
	id, ok := svc.OwnerAgentID()
	if !ok || id != ownerAgentID(identity) {
		t.Fatalf("OwnerAgentID = %d,%v; want %d,true", id, ok, ownerAgentID(identity))
	}

	// Keys stored at the owner's home, hex-encoded and decodable.
	home := []string{"world", "agent", itoa(id)}
	pub, ok := readText(t, svc, append(append([]string{}, home...), "keys", "public"))
	if !ok {
		t.Fatal("public key not stored")
	}
	if b, err := hex.DecodeString(pub); err != nil || len(b) == 0 {
		t.Fatalf("public key not valid hex/non-empty: %v", err)
	}
	priv, ok := readText(t, svc, append(append([]string{}, home...), "keys", "private"))
	if !ok {
		t.Fatal("private key not stored")
	}
	if b, err := hex.DecodeString(priv); err != nil || len(b) == 0 {
		t.Fatalf("private key not valid hex/non-empty: %v", err)
	}

	// Device secret file exists with 0600 perms.
	info, err := os.Stat(filepath.Join(svc.storageDir, deviceSecretFile))
	if err != nil {
		t.Fatalf("device secret file missing: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("device secret perms = %o, want 600", perm)
	}
}

func TestInitOwner_AlreadyInitializedFails(t *testing.T) {
	svc := newSeedTestService(t)
	if _, err := svc.InitOwner("first"); err != nil {
		t.Fatalf("first InitOwner: %v", err)
	}
	if _, err := svc.InitOwner("second"); err == nil {
		t.Fatal("expected error initializing an already-owned node")
	}
}

func TestInitOwner_EmptyPassphraseFails(t *testing.T) {
	svc := newSeedTestService(t)
	if _, err := svc.InitOwner(""); err == nil {
		t.Fatal("expected error for empty passphrase")
	}
}

func TestOwnerAgentID_UninitializedReturnsFalse(t *testing.T) {
	svc := newSeedTestService(t)
	if id, ok := svc.OwnerAgentID(); ok {
		t.Fatalf("expected no owner, got id=%d", id)
	}
}

// itoa avoids pulling strconv into the test just for one conversion.
func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
