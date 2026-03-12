package security

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

func TestStampManager_PersonalStamp(t *testing.T) {
	tree := storage.NewMemoryTree()
	stampManager := NewStampManager(tree)
	agentID := uint64(12345)

	// Set personal stamp
	personalStamp := map[string]interface{}{
		"role":     types.Text{Value: "engineer"},
		"timezone": types.Text{Value: "America/New_York"},
	}

	err := stampManager.SetPersonalStamp(personalStamp, agentID)
	if err != nil {
		t.Fatalf("Failed to set personal stamp: %v", err)
	}

	// Retrieve personal stamp
	retrievedStamp, err := stampManager.getPersonalStamp(agentID)
	if err != nil {
		t.Fatalf("Failed to get personal stamp: %v", err)
	}

	// Verify stamp contents
	if len(retrievedStamp) != 2 {
		t.Errorf("Expected 2 stamp attributes, got %d", len(retrievedStamp))
	}

	if role, exists := retrievedStamp["role"]; !exists {
		t.Error("Expected role in personal stamp")
	} else if roleText, ok := role.(types.Text); !ok || roleText.Value != "engineer" {
		t.Errorf("Expected role to be 'engineer', got %v", role)
	}
}

func TestStampManager_HierarchicalStamp(t *testing.T) {
	tree := storage.NewMemoryTree()
	stampManager := NewStampManager(tree)
	agentID := uint64(12345)

	// Set hierarchical stamp at department level
	departmentPath := []string{"world", "company", "department"}
	departmentStamp := map[string]interface{}{
		"department": types.Text{Value: "IT"},
		"clearance":  types.Text{Value: "internal"},
	}

	err := stampManager.SetHierarchicalStamp(departmentPath, departmentStamp, agentID)
	if err != nil {
		t.Fatalf("Failed to set hierarchical stamp: %v", err)
	}

	// Retrieve hierarchical stamp
	retrievedStamp, err := stampManager.getHierarchicalStamp(departmentPath, agentID)
	if err != nil {
		t.Fatalf("Failed to get hierarchical stamp: %v", err)
	}

	// Verify stamp contents
	if len(retrievedStamp) != 2 {
		t.Errorf("Expected 2 stamp attributes, got %d", len(retrievedStamp))
	}

	if dept, exists := retrievedStamp["department"]; !exists {
		t.Error("Expected department in hierarchical stamp")
	} else if deptText, ok := dept.(types.Text); !ok || deptText.Value != "IT" {
		t.Errorf("Expected department to be 'IT', got %v", dept)
	}
}

func TestStampManager_CollectStamps(t *testing.T) {
	tree := storage.NewMemoryTree()
	stampManager := NewStampManager(tree)
	agentID := uint64(12345)

	// Set personal stamp
	personalStamp := map[string]interface{}{
		"role":     types.Text{Value: "engineer"},
		"timezone": types.Text{Value: "America/New_York"},
	}
	stampManager.SetPersonalStamp(personalStamp, agentID)

	// Set hierarchical stamp at company level
	companyPath := []string{"world", "company"}
	companyStamp := map[string]interface{}{
		"company":          types.Text{Value: "AmeriCU"},
		"employment_type":  types.Text{Value: "full-time"},
	}
	stampManager.SetHierarchicalStamp(companyPath, companyStamp, agentID)

	// Set hierarchical stamp at department level (overrides role)
	departmentPath := []string{"world", "company", "department"}
	departmentStamp := map[string]interface{}{
		"department": types.Text{Value: "IT"},
		"role":       types.Text{Value: "lead"}, // Overrides personal role
	}
	stampManager.SetHierarchicalStamp(departmentPath, departmentStamp, agentID)

	// Collect effective stamp for a write at task level
	writePath := []string{"world", "company", "department", "task42"}
	effectiveStamp, err := stampManager.CollectStamps(writePath, agentID)
	if err != nil {
		t.Fatalf("Failed to collect stamps: %v", err)
	}

	// Verify effective stamp
	expected := map[string]interface{}{
		"@author":          agentID,                               // System-injected
		"role":             types.Text{Value: "lead"},             // Department wins over personal
		"timezone":         types.Text{Value: "America/New_York"}, // From personal
		"company":          types.Text{Value: "AmeriCU"},          // From company
		"employment_type":  types.Text{Value: "full-time"},       // From company
		"department":       types.Text{Value: "IT"},              // From department
	}

	if len(effectiveStamp.Attributes) != len(expected) {
		t.Errorf("Expected %d attributes in effective stamp, got %d",
			len(expected), len(effectiveStamp.Attributes))
	}

	// Check each expected attribute
	for key, expectedValue := range expected {
		if actualValue, exists := effectiveStamp.Attributes[key]; !exists {
			t.Errorf("Expected %s in effective stamp", key)
		} else if !equalStampValues(actualValue, expectedValue) {
			t.Errorf("Expected %s to be %v, got %v", key, expectedValue, actualValue)
		}
	}
}

func TestStampManager_EmbedStamp(t *testing.T) {
	tree := storage.NewMemoryTree()
	stampManager := NewStampManager(tree)

	// Create instance with some attributes
	instance := map[string]interface{}{
		"id":   types.Number{Value: 42},
		"name": types.Text{Value: "Test"},
	}

	// Create stamp
	stamp := &StampSnapshot{
		Attributes: map[string]interface{}{
			"@author":    uint64(12345),
			"role":       types.Text{Value: "engineer"},
			"department": types.Text{Value: "IT"},
			"name":       types.Text{Value: "StampName"}, // Conflicts with instance
		},
	}

	// Embed stamp
	result := stampManager.EmbedStamp(instance, stamp)

	// Verify results
	// Instance attributes should win over stamp attributes
	if name, exists := result["name"]; !exists {
		t.Error("Expected name attribute")
	} else if nameText, ok := name.(types.Text); !ok || nameText.Value != "Test" {
		t.Errorf("Expected name to be 'Test' (instance wins), got %v", name)
	}

	// Stamp attributes should appear where no conflict
	if role, exists := result["role"]; !exists {
		t.Error("Expected role from stamp")
	} else if roleText, ok := role.(types.Text); !ok || roleText.Value != "engineer" {
		t.Errorf("Expected role to be 'engineer', got %v", role)
	}

	if author, exists := result["@author"]; !exists {
		t.Error("Expected @author from stamp")
	} else if authorID, ok := author.(uint64); !ok || authorID != 12345 {
		t.Errorf("Expected @author to be 12345, got %v", author)
	}
}

func TestStampManager_ProtectedAttributes(t *testing.T) {
	tree := storage.NewMemoryTree()
	stampManager := NewStampManager(tree)

	// Create instance with protected attribute
	instance := map[string]interface{}{
		"id":   CreateProtectedAttribute(types.Number{Value: 42}),
		"name": types.Text{Value: "Test"},
	}

	// Create stamp that tries to override protected attribute
	stamp := &StampSnapshot{
		Attributes: map[string]interface{}{
			"id":   types.Number{Value: 999}, // Tries to override protected
			"role": types.Text{Value: "engineer"},
		},
	}

	// Embed stamp
	result := stampManager.EmbedStamp(instance, stamp)

	// Protected attribute should not be overridden
	if id, exists := result["id"]; !exists {
		t.Error("Expected id attribute")
	} else {
		// Should still be the protected original value
		actualValue := GetAttributeValue(id)
		if idNum, ok := actualValue.(types.Number); !ok || idNum.Value != 42 {
			t.Errorf("Expected protected id to remain 42, got %v", actualValue)
		}
	}

	// Non-protected stamp attribute should be added
	if role, exists := result["role"]; !exists {
		t.Error("Expected role from stamp")
	} else if roleText, ok := role.(types.Text); !ok || roleText.Value != "engineer" {
		t.Errorf("Expected role to be 'engineer', got %v", role)
	}
}

func TestCreateProtectedAttribute(t *testing.T) {
	value := types.Text{Value: "protected value"}
	protected := CreateProtectedAttribute(value)

	// Verify structure
	if _, exists := protected["value"]; !exists {
		t.Error("Expected 'value' field in protected attribute")
	}

	if protectedFlag, exists := protected["@protected"]; !exists {
		t.Error("Expected '@protected' field")
	} else if flag, ok := protectedFlag.(bool); !ok || !flag {
		t.Errorf("Expected @protected to be true, got %v", protectedFlag)
	}

	// Test value extraction
	extracted := GetAttributeValue(protected)
	if !equalStampValues(extracted, value) {
		t.Errorf("Expected extracted value to be %v, got %v", value, extracted)
	}
}

func TestStampManager_StampCaching(t *testing.T) {
	tree := storage.NewMemoryTree()
	stampManager := NewStampManager(tree)
	agentID := uint64(12345)

	// Set up stamps
	personalStamp := map[string]interface{}{
		"role": types.Text{Value: "engineer"},
	}
	stampManager.SetPersonalStamp(personalStamp, agentID)

	writePath := []string{"world", "test"}

	// Collect stamps twice
	stamp1, err := stampManager.CollectStamps(writePath, agentID)
	if err != nil {
		t.Fatalf("Failed to collect stamps first time: %v", err)
	}

	stamp2, err := stampManager.CollectStamps(writePath, agentID)
	if err != nil {
		t.Fatalf("Failed to collect stamps second time: %v", err)
	}

	// Should return the same snapshot (cached)
	if stamp1.Hash != stamp2.Hash {
		t.Error("Expected cached stamp snapshots to have same hash")
	}
}

// Helper function to compare stamp values
func equalStampValues(a, b interface{}) bool {
	switch va := a.(type) {
	case types.Text:
		if vb, ok := b.(types.Text); ok {
			return va.Value == vb.Value
		}
	case types.Number:
		if vb, ok := b.(types.Number); ok {
			return va.Value == vb.Value
		}
	case uint64:
		if vb, ok := b.(uint64); ok {
			return va == vb
		}
	}
	return false
}
