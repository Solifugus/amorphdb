package service

import (
	"path/filepath"
	"testing"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// newSeedTestService builds a Service backed by an isolated temp storage dir.
// New() runs seedBaseStructure() as part of construction, so the root
// scaffolding is present without needing to Start() the service.
func newSeedTestService(t *testing.T) *Service {
	t.Helper()
	cfg := DefaultConfig()
	cfg.StorageDir = t.TempDir()
	cfg.LocalSocketPath = filepath.Join(t.TempDir(), "sock")
	cfg.NetworkPort = 0
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return svc
}

// TestSeedBaseStructure_CreatesRootScaffolding verifies that world and
// world.agent are materialized on service construction. Without them, the
// interpreter's intermediate-path creation is rejected at the [world] ancestor
// (world @write defaults closed) and no agent can create its home.
func TestSeedBaseStructure_CreatesRootScaffolding(t *testing.T) {
	svc := newSeedTestService(t)

	for _, path := range [][]string{{"world"}, {"world", "agent"}} {
		v, err := svc.tree.Read(path)
		if err != nil {
			t.Fatalf("expected %v to exist after New, got error: %v", path, err)
		}
		if v.TypeTag == types.TypeNothing {
			t.Fatalf("expected %v to be a real node, got Nothing", path)
		}
	}
}

// TestSeedBaseStructure_IdempotentAndNonClobbering verifies that re-seeding an
// already-initialized tree neither errors nor overwrites existing agent data
// stored beneath the scaffolding.
func TestSeedBaseStructure_IdempotentAndNonClobbering(t *testing.T) {
	svc := newSeedTestService(t)

	note := types.Text{Value: "keepme"}
	val := storage.Value{TypeTag: note.TypeTag(), Data: note.Serialize()}
	if err := svc.tree.Write([]string{"world", "agent", "42", "note"}, val, 42); err != nil {
		t.Fatalf("write agent data: %v", err)
	}

	if err := svc.seedBaseStructure(); err != nil {
		t.Fatalf("re-seed returned error: %v", err)
	}

	got, err := svc.tree.Read([]string{"world", "agent", "42", "note"})
	if err != nil {
		t.Fatalf("read agent data after re-seed: %v", err)
	}
	if got.TypeTag != note.TypeTag() || string(got.Data) != string(val.Data) {
		t.Fatalf("agent data changed after re-seed: got %+v, want %+v", got, val)
	}
}
