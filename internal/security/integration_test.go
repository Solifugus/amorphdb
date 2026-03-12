// Integration test demonstrating the complete Step 8 security system:
// Permissions + Stamps + Filters working together
package security

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

func TestSecurityIntegration_CompleteWorkflow(t *testing.T) {
	tree := storage.NewMemoryTree()

	// Create security components
	permEval := NewPermissionEvaluator(tree)
	stampManager := NewStampManager(tree)
	filterManager := NewFilterManager(tree, permEval)

	// Create test agents
	engineerID := uint64(12345)
	managerID := uint64(67890)
	adminID := uint64(99999)

	engineer := &Agent{
		Identity: "kalevo",
		AgentID:  engineerID,
		Tree:     tree,
		Stamp: map[string]interface{}{
			"role":       types.Text{Value: "engineer"},
			"clearance":  types.Text{Value: "internal"},
			"department": types.Text{Value: "IT"},
		},
	}

	manager := &Agent{
		Identity: "miratu",
		AgentID:  managerID,
		Tree:     tree,
		Stamp: map[string]interface{}{
			"role":       types.Text{Value: "manager"},
			"clearance":  types.Text{Value: "internal"},
			"department": types.Text{Value: "IT"},
		},
	}

	admin := &Agent{
		Identity: "admin",
		AgentID:  adminID,
		Tree:     tree,
		Stamp: map[string]interface{}{
			"role":      types.Text{Value: "admin"},
			"clearance": types.Text{Value: "admin"},
		},
	}

	// === SETUP STAMPS ===

	// Set personal stamps
	stampManager.SetPersonalStamp(map[string]interface{}{
		"role":      types.Text{Value: "engineer"},
		"timezone":  types.Text{Value: "America/New_York"},
	}, engineerID)

	stampManager.SetPersonalStamp(map[string]interface{}{
		"role":      types.Text{Value: "manager"},
		"timezone":  types.Text{Value: "America/Los_Angeles"},
	}, managerID)

	// Set hierarchical stamps
	departmentPath := []string{"world", "company", "department"}
	stampManager.SetHierarchicalStamp(departmentPath, map[string]interface{}{
		"department": types.Text{Value: "IT"},
		"clearance":  types.Text{Value: "internal"},
		"budget":     types.Number{Value: 100000},
	}, adminID)

	// === SETUP PERMISSIONS ===

	// Department level permissions
	permEval.SetPermission(departmentPath, ReadPermission, types.Text{Value: "(Anything)"}, adminID)
	permEval.SetPermission(departmentPath, WritePermission, types.Text{Value: "(Anything)"}, adminID) // For testing

	// Secrets subdirectory - restricted
	secretsPath := []string{"world", "company", "department", "secrets"}
	permEval.SetPermission(secretsPath, ReadPermission, types.Text{Value: "(Nothing)"}, adminID)
	permEval.SetPermission(secretsPath, WritePermission, types.Text{Value: "(Nothing)"}, adminID)

	// === SETUP FILTERS ===

	// Engineer only wants to see internal clearance data
	engineerFilter := CreateClearanceFilter([]string{"internal"})
	filterManager.SetFilter(engineerFilter, engineerID)

	// Manager wants to see all clearance levels
	managerFilter := CreateClearanceFilter([]string{"public", "internal", "admin"})
	filterManager.SetFilter(managerFilter, managerID)

	// === TEST DATA WITH STAMPS ===

	// Simulate writing data with stamps
	taskPath := []string{"world", "company", "department", "task42"}

	// Engineer writes some data
	engineerStamp, err := stampManager.CollectStamps(taskPath, engineerID)
	if err != nil {
		t.Fatalf("Failed to collect engineer stamp: %v", err)
	}

	// Create data instances with embedded stamps
	dataInstances := map[string]interface{}{
		"public_report": stampManager.EmbedStamp(map[string]interface{}{
			"title": types.Text{Value: "Public Report"},
			"data":  types.Text{Value: "Public data"},
		}, &StampSnapshot{
			Attributes: map[string]interface{}{
				"@author":    engineerID,
				"clearance":  types.Text{Value: "public"},
				"department": types.Text{Value: "IT"},
			},
		}),
		"internal_report": stampManager.EmbedStamp(map[string]interface{}{
			"title": types.Text{Value: "Internal Report"},
			"data":  types.Text{Value: "Internal data"},
		}, engineerStamp),
		"secret_document": stampManager.EmbedStamp(map[string]interface{}{
			"title": types.Text{Value: "Secret Document"},
			"data":  types.Text{Value: "Secret data"},
		}, &StampSnapshot{
			Attributes: map[string]interface{}{
				"@author":    adminID,
				"clearance":  types.Text{Value: "admin"},
				"department": types.Text{Value: "IT"},
			},
		}),
	}

	// === TEST PERMISSIONS ===

	// Test read permissions
	canEngineerRead := permEval.CanRead(engineer, departmentPath)
	if !canEngineerRead {
		t.Error("Engineer should be able to read department data")
	}

	canEngineerReadSecrets := permEval.CanRead(engineer, secretsPath)
	if canEngineerReadSecrets {
		t.Error("Engineer should NOT be able to read secrets")
	}

	// === TEST FILTERS ===

	// Engineer's filtered view (only internal clearance)
	engineerView, err := filterManager.FilterVisible(engineer, dataInstances)
	if err != nil {
		t.Fatalf("Failed to get engineer view: %v", err)
	}

	// Engineer should only see internal_report (clearance: internal)
	if len(engineerView) != 1 {
		t.Errorf("Engineer should see 1 item, got %d", len(engineerView))
	}

	if _, exists := engineerView["internal_report"]; !exists {
		t.Error("Engineer should see internal_report")
	}

	if _, exists := engineerView["public_report"]; exists {
		t.Error("Engineer should NOT see public_report (filter excludes public)")
	}

	// Manager's filtered view (all clearance levels)
	managerView, err := filterManager.FilterVisible(manager, dataInstances)
	if err != nil {
		t.Fatalf("Failed to get manager view: %v", err)
	}

	// Manager should see all 3 items (filter allows all clearance levels)
	if len(managerView) != 3 {
		t.Errorf("Manager should see 3 items, got %d", len(managerView))
	}

	// === TEST STAMP EMBEDDING ===

	// Verify stamp attributes are embedded in instances
	if internalReport, exists := dataInstances["internal_report"]; exists {
		if reportMap, ok := internalReport.(map[string]interface{}); ok {
			// Should have @author from stamp
			if author, exists := reportMap["@author"]; !exists {
				t.Error("Expected @author in stamped instance")
			} else if authorID, ok := author.(uint64); !ok || authorID != engineerID {
				t.Errorf("Expected @author to be %d, got %v", engineerID, author)
			}

			// Should have department from hierarchical stamp
			if dept, exists := reportMap["department"]; !exists {
				t.Error("Expected department in stamped instance")
			} else if deptText, ok := dept.(types.Text); !ok || deptText.Value != "IT" {
				t.Errorf("Expected department to be 'IT', got %v", dept)
			}
		}
	}

	// === TEST COMPLETE SECURITY WORKFLOW ===

	// Scenario: Engineer tries to access data
	// 1. Check permissions (can read department)
	// 2. Apply stamps (data has embedded context)
	// 3. Apply filters (only see internal clearance)

	canAccess := permEval.CanRead(engineer, departmentPath)
	if !canAccess {
		t.Error("Engineer should have read permission")
	}

	// Get filtered view of the data
	engineerFiltered, _ := filterManager.FilterVisible(engineer, dataInstances)

	// Verify engineer gets the right view
	if len(engineerFiltered) != 1 {
		t.Errorf("Engineer's complete workflow should show 1 item, got %d", len(engineerFiltered))
	}
}

func TestSecurityIntegration_ProtectedAttributes(t *testing.T) {
	tree := storage.NewMemoryTree()
	stampManager := NewStampManager(tree)
	agentID := uint64(12345)

	// Create instance with protected attribute
	instance := map[string]interface{}{
		"id":   CreateProtectedAttribute(types.Number{Value: 42}),
		"name": types.Text{Value: "Test Instance"},
	}

	// Create stamp that tries to override both attributes
	stamp := &StampSnapshot{
		Attributes: map[string]interface{}{
			"id":         types.Number{Value: 999}, // Tries to override protected
			"name":       types.Text{Value: "Stamp Name"}, // Tries to override regular
			"@author":    agentID,
			"department": types.Text{Value: "IT"},
		},
	}

	// Embed stamp
	result := stampManager.EmbedStamp(instance, stamp)

	// Protected attribute should not be overridden
	if idAttr, exists := result["id"]; !exists {
		t.Error("Expected id attribute")
	} else {
		actualValue := GetAttributeValue(idAttr)
		if idNum, ok := actualValue.(types.Number); !ok || idNum.Value != 42 {
			t.Errorf("Protected id should remain 42, got %v", actualValue)
		}
	}

	// Regular attribute should also not be overridden (instance wins)
	if nameAttr, exists := result["name"]; !exists {
		t.Error("Expected name attribute")
	} else if nameText, ok := nameAttr.(types.Text); !ok || nameText.Value != "Test Instance" {
		t.Errorf("Instance name should win, expected 'Test Instance', got %v", nameAttr)
	}

	// Stamp-only attributes should be added
	if author, exists := result["@author"]; !exists {
		t.Error("Expected @author from stamp")
	} else if authorID, ok := author.(uint64); !ok || authorID != agentID {
		t.Errorf("Expected @author to be %d, got %v", agentID, author)
	}

	if dept, exists := result["department"]; !exists {
		t.Error("Expected department from stamp")
	} else if deptText, ok := dept.(types.Text); !ok || deptText.Value != "IT" {
		t.Errorf("Expected department to be 'IT', got %v", dept)
	}
}

func TestSecurityIntegration_CascadingPermissions(t *testing.T) {
	tree := storage.NewMemoryTree()
	permEval := NewPermissionEvaluator(tree)
	agentID := uint64(12345)

	agent := &Agent{
		Identity: "kalevo",
		AgentID:  agentID,
		Tree:     tree,
		Stamp: map[string]interface{}{
			"role": types.Text{Value: "employee"},
		},
	}

	// Set permission at company level
	companyPath := []string{"world", "company"}
	permEval.SetPermission(companyPath, ReadPermission, types.Text{Value: "(Anything)"}, agentID)

	// Set more restrictive permission at secrets level
	secretsPath := []string{"world", "company", "department", "secrets"}
	permEval.SetPermission(secretsPath, ReadPermission, types.Text{Value: "(Nothing)"}, agentID)

	// Test inheritance: department should inherit company permission
	departmentPath := []string{"world", "company", "department"}
	canReadDept := permEval.CanRead(agent, departmentPath)
	if !canReadDept {
		t.Error("Should be able to read department (inherits company permission)")
	}

	// Test override: secrets should use its own permission
	canReadSecrets := permEval.CanRead(agent, secretsPath)
	if canReadSecrets {
		t.Error("Should NOT be able to read secrets (overridden permission)")
	}

	// Test deeper inheritance: subsection under department
	subsectionPath := []string{"world", "company", "department", "public", "data"}
	canReadSubsection := permEval.CanRead(agent, subsectionPath)
	if !canReadSubsection {
		t.Error("Should be able to read subsection (inherits department/company permission)")
	}
}
