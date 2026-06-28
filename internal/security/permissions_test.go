package security

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

func TestPermissionEvaluator_DefaultPermissions(t *testing.T) {
	tree := storage.NewMemoryTree()
	evaluator := NewPermissionEvaluator(tree)

	// Create test agent
	agent := &Agent{
		Identity: "kalevo",
		Stamp: map[string]interface{}{
			"role":      types.Text{Value: "engineer"},
			"clearance": types.Text{Value: "internal"},
		},
		Tree:    tree,
		AgentID: 12345,
	}

	tests := []struct {
		name     string
		path     []string
		permType PermissionType
		expected bool
	}{
		// Under ~ (home) - should default to owner permissions (true for owner)
		{"Home read", []string{"~"}, ReadPermission, true},
		{"Home write", []string{"~"}, WritePermission, true},
		{"Home expand", []string{"~"}, ExpandPermission, true},
		{"Home grant", []string{"~"}, GrantPermission, true},
		{"Home purge", []string{"~"}, PurgePermission, true},

		// Under world - should have specific defaults
		{"World read", []string{"world"}, ReadPermission, true},      // Open by default
		{"World write", []string{"world"}, WritePermission, false},   // Closed by default
		{"World expand", []string{"world"}, ExpandPermission, false}, // Closed by default
		{"World grant", []string{"world"}, GrantPermission, false},   // Closed by default
		{"World purge", []string{"world"}, PurgePermission, false},   // Closed by default

		// Deep paths under world
		{"Deep world read", []string{"world", "data", "public"}, ReadPermission, true},
		{"Deep world write", []string{"world", "data", "public"}, WritePermission, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.CheckPermission(agent, tt.path, tt.permType)
			if err != nil {
				t.Errorf("CheckPermission() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("CheckPermission() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPermissionEvaluator_ExplicitPermissions(t *testing.T) {
	tree := storage.NewMemoryTree()
	evaluator := NewPermissionEvaluator(tree)

	// Create test agent
	agent := &Agent{
		Identity: "kalevo",
		Stamp: map[string]interface{}{
			"role":      types.Text{Value: "engineer"},
			"clearance": types.Text{Value: "internal"},
		},
		Tree:    tree,
		AgentID: 12345,
	}

	// Set explicit permissions
	departmentPath := []string{"world", "company", "department"}

	// Set read permission to allow engineers
	err := evaluator.SetPermission(departmentPath, ReadPermission, types.Text{Value: "(Anything)"}, agent.AgentID)
	if err != nil {
		t.Fatalf("Failed to set read permission: %v", err)
	}

	// Set write permission to deny all
	err = evaluator.SetPermission(departmentPath, WritePermission, types.Text{Value: "(Nothing)"}, agent.AgentID)
	if err != nil {
		t.Fatalf("Failed to set write permission: %v", err)
	}

	// Test the explicit permissions
	canRead, err := evaluator.CheckPermission(agent, departmentPath, ReadPermission)
	if err != nil {
		t.Errorf("CheckPermission(read) error = %v", err)
	}
	if !canRead {
		t.Errorf("Expected read permission to be true")
	}

	canWrite, err := evaluator.CheckPermission(agent, departmentPath, WritePermission)
	if err != nil {
		t.Errorf("CheckPermission(write) error = %v", err)
	}
	if canWrite {
		t.Errorf("Expected write permission to be false")
	}
}

func TestPermissionEvaluator_PermissionCascade(t *testing.T) {
	tree := storage.NewMemoryTree()
	evaluator := NewPermissionEvaluator(tree)

	// Create test agent
	agent := &Agent{
		Identity: "kalevo",
		Stamp: map[string]interface{}{
			"role": types.Text{Value: "engineer"},
		},
		Tree:    tree,
		AgentID: 12345,
	}

	// Set permission at department level
	departmentPath := []string{"world", "company", "department"}
	err := evaluator.SetPermission(departmentPath, ReadPermission, types.Text{Value: "(Anything)"}, agent.AgentID)
	if err != nil {
		t.Fatalf("Failed to set department permission: %v", err)
	}

	// Set more restrictive permission at secrets sublevel
	secretsPath := []string{"world", "company", "department", "secrets"}
	err = evaluator.SetPermission(secretsPath, ReadPermission, types.Text{Value: "(Nothing)"}, agent.AgentID)
	if err != nil {
		t.Fatalf("Failed to set secrets permission: %v", err)
	}

	// Test cascade - public should inherit department permission (open)
	publicPath := []string{"world", "company", "department", "public"}
	canReadPublic, err := evaluator.CheckPermission(agent, publicPath, ReadPermission)
	if err != nil {
		t.Errorf("CheckPermission(public) error = %v", err)
	}
	if !canReadPublic {
		t.Errorf("Expected public read to inherit department permission (true)")
	}

	// Test override - secrets should use its own permission (closed)
	canReadSecrets, err := evaluator.CheckPermission(agent, secretsPath, ReadPermission)
	if err != nil {
		t.Errorf("CheckPermission(secrets) error = %v", err)
	}
	if canReadSecrets {
		t.Errorf("Expected secrets read to be false (overridden)")
	}
}

// TestPermissionEvaluator_TreeAdapterGrantRoundTrip is the regression guard for
// the production storage.TreeAdapter. The other explicit-permission tests use
// storage.NewMemoryTree(), which has always round-tripped its hash-keyed
// metadata; the TreeAdapter used by the live daemon previously faked that
// metadata with dummy paths and silently dropped every grant, so a granted
// world.* @write was never honored over the protocol. This test exercises the
// real adapter end-to-end: world.* write is denied by default, allowed after an
// explicit grant, and unrelated world.* paths stay denied.
func TestPermissionEvaluator_TreeAdapterGrantRoundTrip(t *testing.T) {
	st, err := storage.NewStorageTree(t.TempDir())
	if err != nil {
		t.Fatalf("NewStorageTree failed: %v", err)
	}
	tree := storage.NewTreeAdapter(st)
	evaluator := NewPermissionEvaluator(tree)

	agent := &Agent{
		Identity: "kalevo",
		Stamp:    map[string]interface{}{},
		Tree:     tree,
		AgentID:  12345,
	}

	grantedPath := []string{"world", "projects", "alpha"}
	otherPath := []string{"world", "projects", "beta"}

	// Default: world.* write is closed.
	if evaluator.CanWrite(agent, grantedPath) {
		t.Fatalf("expected world.projects.alpha write to be denied by default")
	}

	// Grant write on the path.
	if err := evaluator.SetPermission(grantedPath, WritePermission, types.Text{Value: "(Anything)"}, agent.AgentID); err != nil {
		t.Fatalf("SetPermission failed: %v", err)
	}

	// The grant must now be honored via the production adapter's round-trip.
	if !evaluator.CanWrite(agent, grantedPath) {
		t.Errorf("expected world.projects.alpha write to be allowed after explicit grant")
	}

	// An unrelated world.* path must remain denied (grant did not leak).
	if evaluator.CanWrite(agent, otherPath) {
		t.Errorf("expected world.projects.beta write to remain denied (no grant)")
	}
}

func TestAgent_GetProperty(t *testing.T) {
	agent := &Agent{
		Identity: "kalevo",
		Stamp: map[string]interface{}{
			"role":       types.Text{Value: "engineer"},
			"clearance":  types.Text{Value: "internal"},
			"department": types.Text{Value: "IT"},
		},
	}

	tests := []struct {
		name     string
		property string
		expected interface{}
	}{
		{"Identity property", "identity", types.Text{Value: "kalevo"}},
		{"Role from stamp", "role", types.Text{Value: "engineer"}},
		{"Clearance from stamp", "clearance", types.Text{Value: "internal"}},
		{"Department from stamp", "department", types.Text{Value: "IT"}},
		{"Non-existent property", "nonexistent", types.Nothing{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.GetProperty(tt.property)
			if !equalValues(result, tt.expected) {
				t.Errorf("GetProperty(%s) = %v, want %v", tt.property, result, tt.expected)
			}
		})
	}
}

func TestPermissionEvaluator_ConvenienceMethods(t *testing.T) {
	tree := storage.NewMemoryTree()
	evaluator := NewPermissionEvaluator(tree)

	agent := &Agent{
		Identity: "kalevo",
		Stamp: map[string]interface{}{
			"role": types.Text{Value: "admin"},
		},
		Tree:    tree,
		AgentID: 12345,
	}

	path := []string{"world", "test"}

	// Set various permissions
	evaluator.SetPermission(path, ReadPermission, types.Text{Value: "(Anything)"}, agent.AgentID)
	evaluator.SetPermission(path, WritePermission, types.Text{Value: "(Nothing)"}, agent.AgentID)
	evaluator.SetPermission(path, ExpandPermission, types.Text{Value: "(Anything)"}, agent.AgentID)
	evaluator.SetPermission(path, GrantPermission, types.Text{Value: "(Nothing)"}, agent.AgentID)
	evaluator.SetPermission(path, PurgePermission, types.Text{Value: "(Anything)"}, agent.AgentID)

	// Test convenience methods
	if !evaluator.CanRead(agent, path) {
		t.Error("Expected CanRead to be true")
	}
	if evaluator.CanWrite(agent, path) {
		t.Error("Expected CanWrite to be false")
	}
	if !evaluator.CanExpand(agent, path) {
		t.Error("Expected CanExpand to be true")
	}
	if evaluator.CanGrant(agent, path) {
		t.Error("Expected CanGrant to be false")
	}
	if !evaluator.CanPurge(agent, path) {
		t.Error("Expected CanPurge to be true")
	}
}

// Helper function to compare values
func equalValues(a, b interface{}) bool {
	switch va := a.(type) {
	case types.Text:
		if vb, ok := b.(types.Text); ok {
			return va.Value == vb.Value
		}
	case types.Nothing:
		_, ok := b.(types.Nothing)
		return ok
	case types.Boolean:
		if vb, ok := b.(types.Boolean); ok {
			return va.Value == vb.Value
		}
	}
	return false
}
