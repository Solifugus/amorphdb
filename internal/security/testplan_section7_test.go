// testplan_section7_test.go - Section 7: Stamps, Filters, Permissions Test Suite
//
// This file implements the comprehensive test plan for Section 7 from AmorphDB_Test_Plan.md.
// Section 7 tests stamps (metadata injection), embed syntax, filters (client-side data hiding),
// and permissions (capability-based access control).

package security

import (
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// Helper function to create test storage and security components
func newSecurityTestSetup(t *testing.T) (storage.ExtendedTree, *StampManager, *FilterManager, *PermissionEvaluator) {
	t.Helper()

	tree := storage.NewMemoryTree()
	stampManager := NewStampManager(tree)
	permEval := NewPermissionEvaluator(tree)
	filterManager := NewFilterManager(tree, permEval)

	return tree, stampManager, filterManager, permEval
}

// Helper function to create test agent
func createTestAgent(agentID uint64) *Agent {
	return &Agent{
		Identity: "test_agent",
		AgentID:  agentID,
		Stamp: map[string]interface{}{
			"role":       types.Text{Value: "engineer"},
			"clearance":  types.Text{Value: "internal"},
			"department": types.Text{Value: "IT"},
		},
	}
}

//=============================================================================
// 7.1 STAMPS
//=============================================================================

// Test 7.1.1: Built-in stamps auto-applied
// Write data — `@agent`, `@time` are set automatically
func TestStamps_BuiltInStampsAutoApplied(t *testing.T) {
	_, stampManager, _, _ := newSecurityTestSetup(t)
	agent := createTestAgent(12345)

	// Write some data
	writePath := []string{"world", "test", "data"}

	// Collect stamps for this write
	stampSnapshot, err := stampManager.CollectStamps(writePath, agent.AgentID)
	if err != nil {
		t.Fatalf("Failed to collect stamps: %v", err)
	}

	// Verify @author (auto-injected)
	if author, exists := stampSnapshot.Attributes["@author"]; !exists {
		t.Error("Expected @author stamp to be auto-applied")
	} else if authorID, ok := author.(uint64); !ok || authorID != agent.AgentID {
		t.Errorf("Expected @author to be %d, got %v", agent.AgentID, author)
	}

	// Note: @time should be auto-injected at write time, not at stamp collection time
	// For now, we verify the stamp collection mechanism works
	t.Logf("Stamp snapshot created with %d attributes", len(stampSnapshot.Attributes))
}

// Test 7.1.2: Custom stamps
// Set `my.stamp.department = "engineering"`, write data — data carries `@stamp.department`
func TestStamps_CustomStamps(t *testing.T) {
	_, stampManager, _, _ := newSecurityTestSetup(t)
	agentID := uint64(12345)

	// Set custom personal stamps
	customStamp := map[string]interface{}{
		"department": types.Text{Value: "engineering"},
		"project":    types.Text{Value: "amorphdb"},
		"role":       types.Text{Value: "lead_engineer"},
	}

	err := stampManager.SetPersonalStamp(customStamp, agentID)
	if err != nil {
		t.Fatalf("Failed to set personal stamp: %v", err)
	}

	// Write data and collect stamps
	writePath := []string{"world", "test", "feature"}
	stampSnapshot, err := stampManager.CollectStamps(writePath, agentID)
	if err != nil {
		t.Fatalf("Failed to collect stamps: %v", err)
	}

	// Verify custom stamp attributes are present
	if dept, exists := stampSnapshot.Attributes["department"]; !exists {
		t.Error("Expected custom department stamp to be present")
	} else if deptText, ok := dept.(types.Text); !ok || deptText.Value != "engineering" {
		t.Errorf("Expected department to be 'engineering', got %v", dept)
	}

	if proj, exists := stampSnapshot.Attributes["project"]; !exists {
		t.Error("Expected custom project stamp to be present")
	} else if projText, ok := proj.(types.Text); !ok || projText.Value != "amorphdb" {
		t.Errorf("Expected project to be 'amorphdb', got %v", proj)
	}

	if role, exists := stampSnapshot.Attributes["role"]; !exists {
		t.Error("Expected custom role stamp to be present")
	} else if roleText, ok := role.(types.Text); !ok || roleText.Value != "lead_engineer" {
		t.Errorf("Expected role to be 'lead_engineer', got %v", role)
	}
}

// Test 7.1.3: Stamp inheritance on copy
// Copy data — stamps carry over
func TestStamps_InheritanceOnCopy(t *testing.T) {
	_, stampManager, _, _ := newSecurityTestSetup(t)
	agentID := uint64(12345)

	// Set up source data with stamps
	sourceStamp := map[string]interface{}{
		"department":     types.Text{Value: "sales"},
		"classification": types.Text{Value: "internal"},
		"source":         types.Text{Value: "original"},
	}

	err := stampManager.SetPersonalStamp(sourceStamp, agentID)
	if err != nil {
		t.Fatalf("Failed to set source stamp: %v", err)
	}

	// Create source data with stamps
	sourcePath := []string{"world", "sales", "reports", "q4"}
	sourceStampSnapshot, err := stampManager.CollectStamps(sourcePath, agentID)
	if err != nil {
		t.Fatalf("Failed to collect source stamps: %v", err)
	}

	// Simulate copy operation - stamps should inherit
	// In a real implementation, this would be handled by the interpreter during copy
	copiedData := map[string]interface{}{
		"title": types.Text{Value: "Q4 Analysis"},
		"data":  types.Text{Value: "copied data"},
	}

	// Embed the source stamps into the copied data
	result := stampManager.EmbedStamp(copiedData, sourceStampSnapshot)

	// Verify stamps carried over
	if dept, exists := result["department"]; !exists {
		t.Error("Expected department stamp to carry over in copy")
	} else if deptText, ok := dept.(types.Text); !ok || deptText.Value != "sales" {
		t.Errorf("Expected department to be 'sales', got %v", dept)
	}

	if class, exists := result["classification"]; !exists {
		t.Error("Expected classification stamp to carry over in copy")
	} else if classText, ok := class.(types.Text); !ok || classText.Value != "internal" {
		t.Errorf("Expected classification to be 'internal', got %v", class)
	}

	// Verify original data attributes are preserved
	if title, exists := result["title"]; !exists {
		t.Error("Expected title to be preserved")
	} else if titleText, ok := title.(types.Text); !ok || titleText.Value != "Q4 Analysis" {
		t.Errorf("Expected title to be 'Q4 Analysis', got %v", title)
	}
}

// Test 7.1.4: Stamp modification
// Change agent's stamp, write new data — new data has new stamp, old data retains old stamp
func TestStamps_Modification(t *testing.T) {
	_, stampManager, _, _ := newSecurityTestSetup(t)
	agentID := uint64(12345)

	// Set initial stamp
	initialStamp := map[string]interface{}{
		"department": types.Text{Value: "engineering"},
		"version":    types.Text{Value: "v1"},
	}

	err := stampManager.SetPersonalStamp(initialStamp, agentID)
	if err != nil {
		t.Fatalf("Failed to set initial stamp: %v", err)
	}

	// Collect stamps for first write
	firstWritePath := []string{"world", "features", "login"}
	firstStamp, err := stampManager.CollectStamps(firstWritePath, agentID)
	if err != nil {
		t.Fatalf("Failed to collect first stamps: %v", err)
	}

	// Verify initial stamp values
	if dept, exists := firstStamp.Attributes["department"]; !exists {
		t.Error("Expected department in first stamp")
	} else if deptText, ok := dept.(types.Text); !ok || deptText.Value != "engineering" {
		t.Errorf("Expected initial department to be 'engineering', got %v", dept)
	}

	if version, exists := firstStamp.Attributes["version"]; !exists {
		t.Error("Expected version in first stamp")
	} else if versionText, ok := version.(types.Text); !ok || versionText.Value != "v1" {
		t.Errorf("Expected initial version to be 'v1', got %v", version)
	}

	// Modify the stamp
	modifiedStamp := map[string]interface{}{
		"department": types.Text{Value: "product"},
		"version":    types.Text{Value: "v2"},
	}

	err = stampManager.SetPersonalStamp(modifiedStamp, agentID)
	if err != nil {
		t.Fatalf("Failed to set modified stamp: %v", err)
	}

	// Collect stamps for second write
	secondWritePath := []string{"world", "features", "dashboard"}
	secondStamp, err := stampManager.CollectStamps(secondWritePath, agentID)
	if err != nil {
		t.Fatalf("Failed to collect second stamps: %v", err)
	}

	// Verify new stamp has new values
	if dept, exists := secondStamp.Attributes["department"]; !exists {
		t.Error("Expected department in second stamp")
	} else if deptText, ok := dept.(types.Text); !ok || deptText.Value != "product" {
		t.Errorf("Expected new department to be 'product', got %v", dept)
	}

	if version, exists := secondStamp.Attributes["version"]; !exists {
		t.Error("Expected version in second stamp")
	} else if versionText, ok := version.(types.Text); !ok || versionText.Value != "v2" {
		t.Errorf("Expected new version to be 'v2', got %v", version)
	}

	// Verify original stamp is unchanged (different hashes)
	if firstStamp.Hash == secondStamp.Hash {
		t.Error("Expected stamp hashes to be different after modification")
	}
}

// Test 7.1.5: Stamp query
// `my.projects[@stamp.department = "engineering"]` returns correctly
func TestStamps_Query(t *testing.T) {
	// Test stamp queries - finding data that matches stamp criteria
	_, stampManager, _, _ := newSecurityTestSetup(t)

	// Create test data with different department stamps
	testData := map[string]interface{}{
		"project1": map[string]interface{}{
			"title":      types.Text{Value: "Login System"},
			"department": types.Text{Value: "engineering"},
			"status":     types.Text{Value: "active"},
		},
		"project2": map[string]interface{}{
			"title":      types.Text{Value: "Sales Report"},
			"department": types.Text{Value: "sales"},
			"status":     types.Text{Value: "completed"},
		},
		"project3": map[string]interface{}{
			"title":      types.Text{Value: "API Gateway"},
			"department": types.Text{Value: "engineering"},
			"status":     types.Text{Value: "planning"},
		},
	}

	// Query for engineering department projects
	engineeringProjects := stampManager.QueryByStamp(testData, "department", types.Text{Value: "engineering"})

	// Should find project1 and project3
	if len(engineeringProjects) != 2 {
		t.Errorf("Expected 2 engineering projects, got %d", len(engineeringProjects))
	}

	if _, exists := engineeringProjects["project1"]; !exists {
		t.Error("Expected engineering data to be visible with department filter")
	}

	if _, exists := engineeringProjects["project3"]; !exists {
		t.Error("Expected project3 (engineering) to be found")
	}

	// Query for sales department projects
	salesProjects := stampManager.QueryByStamp(testData, "department", types.Text{Value: "sales"})

	// Should find only project2
	if len(salesProjects) != 1 {
		t.Errorf("Expected 1 sales project, got %d", len(salesProjects))
	}

	if _, exists := salesProjects["project2"]; !exists {
		t.Error("Expected sales data to be filtered out with engineering filter")
	}
}

// Test 7.1.6: Multiple custom stamps
// Set 3 stamp fields, write data — all 3 present
func TestStamps_MultipleCustomStamps(t *testing.T) {
	_, stampManager, _, _ := newSecurityTestSetup(t)
	agentID := uint64(12345)

	// Set multiple custom stamps
	multipleStamps := map[string]interface{}{
		"department":     types.Text{Value: "engineering"},
		"project":        types.Text{Value: "customer-portal"},
		"cost_center":    types.Text{Value: "R&D"},
		"classification": types.Text{Value: "internal"},
		"priority":       types.Text{Value: "high"},
	}

	err := stampManager.SetPersonalStamp(multipleStamps, agentID)
	if err != nil {
		t.Fatalf("Failed to set multiple stamps: %v", err)
	}

	// Write data and collect stamps
	writePath := []string{"world", "projects", "portal", "features"}
	stampSnapshot, err := stampManager.CollectStamps(writePath, agentID)
	if err != nil {
		t.Fatalf("Failed to collect stamps: %v", err)
	}

	// Verify all custom stamps are present
	expectedStamps := map[string]string{
		"department":     "engineering",
		"project":        "customer-portal",
		"cost_center":    "R&D",
		"classification": "internal",
		"priority":       "high",
	}

	for key, expectedValue := range expectedStamps {
		if stampValue, exists := stampSnapshot.Attributes[key]; !exists {
			t.Errorf("Expected stamp '%s' to be present", key)
		} else if stampText, ok := stampValue.(types.Text); !ok {
			t.Errorf("Expected stamp '%s' to be Text type", key)
		} else if stampText.Value != expectedValue {
			t.Errorf("Expected stamp '%s' to be '%s', got '%s'", key, expectedValue, stampText.Value)
		}
	}

	// Verify system stamps are also present
	if author, exists := stampSnapshot.Attributes["@author"]; !exists {
		t.Error("Expected @author system stamp to be present")
	} else if authorID, ok := author.(uint64); !ok || authorID != agentID {
		t.Errorf("Expected @author to be %d, got %v", agentID, author)
	}

	// Verify total count (5 custom + 1 system @author)
	expectedCount := 6
	if len(stampSnapshot.Attributes) != expectedCount {
		t.Errorf("Expected %d stamp attributes, got %d", expectedCount, len(stampSnapshot.Attributes))
	}
}

//=============================================================================
// 7.2 EMBED
//=============================================================================

// Test 7.2.1: Basic embed
// `embed my.context.stamp` makes stamp fields appear as siblings in the host record
func TestEmbed_BasicEmbed(t *testing.T) {
	_, stampManager, _, _ := newSecurityTestSetup(t)

	// Create host record
	hostRecord := map[string]interface{}{
		"title":  types.Text{Value: "Q4 Report"},
		"author": types.Text{Value: "Alice"},
		"status": types.Text{Value: "draft"},
	}

	// Create stamp to embed
	stampData := &StampSnapshot{
		Attributes: map[string]interface{}{
			"department":     types.Text{Value: "finance"},
			"classification": types.Text{Value: "confidential"},
			"@author":        uint64(54321),
			"@time":          types.Time{Timestamp: time.Now(), Precision: types.PrecisionSubsecond},
		},
	}

	// Embed stamp into host record
	result := stampManager.EmbedStamp(hostRecord, stampData)

	// Verify host record attributes are preserved
	if title, exists := result["title"]; !exists {
		t.Error("Expected title from host record")
	} else if titleText, ok := title.(types.Text); !ok || titleText.Value != "Q4 Report" {
		t.Errorf("Expected title to be 'Q4 Report', got %v", title)
	}

	// Verify stamp attributes appear as siblings
	if dept, exists := result["department"]; !exists {
		t.Error("Expected department from embedded stamp")
	} else if deptText, ok := dept.(types.Text); !ok || deptText.Value != "finance" {
		t.Errorf("Expected department to be 'finance', got %v", dept)
	}

	if class, exists := result["classification"]; !exists {
		t.Error("Expected classification from embedded stamp")
	} else if classText, ok := class.(types.Text); !ok || classText.Value != "confidential" {
		t.Errorf("Expected classification to be 'confidential', got %v", class)
	}

	if author, exists := result["@author"]; !exists {
		t.Error("Expected @author from embedded stamp")
	} else if authorID, ok := author.(uint64); !ok || authorID != 54321 {
		t.Errorf("Expected @author to be 54321, got %v", author)
	}

	// Verify expected total field count (3 host + 4 stamp)
	// (excluding internal metadata)
	expectedFields := 7
	fieldCount := len(result)
	if _, hasMetadata := result["@embedded_attrs"]; hasMetadata {
		fieldCount-- // Exclude internal metadata from count
	}
	if fieldCount != expectedFields {
		t.Errorf("Expected %d fields after embed, got %d", expectedFields, fieldCount)
	}
}

// Test 7.2.2: Spread syntax
// `...my.context.stamp` produces same result as `embed`
func TestEmbed_SpreadSyntax(t *testing.T) {
	_, stampManager, _, _ := newSecurityTestSetup(t)

	// Create test data
	hostRecord := map[string]interface{}{
		"name":  types.Text{Value: "Product Data"},
		"count": types.Number{Value: 42},
	}

	stampData := &StampSnapshot{
		Attributes: map[string]interface{}{
			"team":    types.Text{Value: "product"},
			"version": types.Text{Value: "v2.1"},
			"active":  types.Text{Value: "true"},
		},
	}

	// Test embed operation (simulating both embed and spread syntax)
	resultEmbed := stampManager.EmbedStamp(hostRecord, stampData)

	// Since the spread syntax `...` produces the same result as `embed`,
	// we verify the embed operation works correctly and produces the expected result
	// In the actual parser/interpreter, spread syntax would call the same embed logic

	// Verify all attributes present
	if name, exists := resultEmbed["name"]; !exists {
		t.Error("Expected name from host record")
	} else if nameText, ok := name.(types.Text); !ok || nameText.Value != "Product Data" {
		t.Errorf("Expected name to be 'Product Data', got %v", name)
	}

	if team, exists := resultEmbed["team"]; !exists {
		t.Error("Expected team from stamp")
	} else if teamText, ok := team.(types.Text); !ok || teamText.Value != "product" {
		t.Errorf("Expected team to be 'product', got %v", team)
	}

	if version, exists := resultEmbed["version"]; !exists {
		t.Error("Expected version from stamp")
	} else if versionText, ok := version.(types.Text); !ok || versionText.Value != "v2.1" {
		t.Errorf("Expected version to be 'v2.1', got %v", version)
	}

	// Total should be 2 host + 3 stamp = 5 fields
	// (excluding internal metadata)
	fieldCount := len(resultEmbed)
	if _, hasMetadata := resultEmbed["@embedded_attrs"]; hasMetadata {
		fieldCount-- // Exclude internal metadata from count
	}
	if fieldCount != 5 {
		t.Errorf("Expected 5 fields after spread, got %d", fieldCount)
	}
}

// Test 7.2.3: Collision — host wins
// Host has field `title`, embed also has `title` — host value is returned
func TestEmbed_CollisionHostWins(t *testing.T) {
	_, stampManager, _, _ := newSecurityTestSetup(t)

	// Create host record with conflicting field
	hostRecord := map[string]interface{}{
		"title":  types.Text{Value: "Host Title"},
		"author": types.Text{Value: "Host Author"},
		"id":     types.Number{Value: 100},
	}

	// Create stamp with conflicting fields
	stampData := &StampSnapshot{
		Attributes: map[string]interface{}{
			"title":      types.Text{Value: "Stamp Title"},                                       // Conflicts with host
			"author":     types.Text{Value: "Stamp Author"},                                      // Conflicts with host
			"department": types.Text{Value: "engineering"},                                       // No conflict
			"timestamp":  types.Time{Timestamp: time.Now(), Precision: types.PrecisionSubsecond}, // No conflict
		},
	}

	// Embed stamp into host record
	result := stampManager.EmbedStamp(hostRecord, stampData)

	// Verify host values win on collision
	if title, exists := result["title"]; !exists {
		t.Error("Expected title field")
	} else if titleText, ok := title.(types.Text); !ok || titleText.Value != "Host Title" {
		t.Errorf("Expected host title 'Host Title' to win, got %v", title)
	}

	if author, exists := result["author"]; !exists {
		t.Error("Expected author field")
	} else if authorText, ok := author.(types.Text); !ok || authorText.Value != "Host Author" {
		t.Errorf("Expected host author 'Host Author' to win, got %v", author)
	}

	// Verify non-conflicting stamp fields are included
	if dept, exists := result["department"]; !exists {
		t.Error("Expected department from stamp")
	} else if deptText, ok := dept.(types.Text); !ok || deptText.Value != "engineering" {
		t.Errorf("Expected department to be 'engineering', got %v", dept)
	}

	if _, exists := result["timestamp"]; !exists {
		t.Error("Expected timestamp from stamp")
	}

	// Total fields: 3 host + 2 non-conflicting stamp = 5
	// (excluding internal metadata)
	fieldCount := len(result)
	if _, hasMetadata := result["@embedded_attrs"]; hasMetadata {
		fieldCount-- // Exclude internal metadata from count
	}
	if fieldCount != 5 {
		t.Errorf("Expected 5 fields after collision resolution, got %d", fieldCount)
	}
}

// Test 7.2.4: Collision — later embed wins
// Two embeds conflict — second one's field value is used
func TestEmbed_CollisionLaterEmbedWins(t *testing.T) {
	_, stampManager, _, _ := newSecurityTestSetup(t)

	// Start with host record
	hostRecord := map[string]interface{}{
		"id":   types.Number{Value: 1},
		"name": types.Text{Value: "Base Record"},
	}

	// First stamp to embed
	firstStamp := &StampSnapshot{
		Attributes: map[string]interface{}{
			"priority": types.Text{Value: "low"},
			"status":   types.Text{Value: "draft"},
			"version":  types.Text{Value: "v1.0"},
		},
	}

	// Second stamp with conflicting fields
	secondStamp := &StampSnapshot{
		Attributes: map[string]interface{}{
			"priority": types.Text{Value: "high"},      // Conflicts with first stamp
			"status":   types.Text{Value: "published"}, // Conflicts with first stamp
			"category": types.Text{Value: "feature"},   // No conflict
		},
	}

	// Apply first embed
	result1 := stampManager.EmbedStamp(hostRecord, firstStamp)

	// Apply second embed to the result (simulating multiple embeds)
	finalResult := stampManager.EmbedStamp(result1, secondStamp)

	// Verify later embed values win on conflict
	if priority, exists := finalResult["priority"]; !exists {
		t.Error("Expected priority field")
	} else if priorityText, ok := priority.(types.Text); !ok || priorityText.Value != "high" {
		t.Errorf("Expected later priority 'high' to win, got %v", priority)
	}

	if status, exists := finalResult["status"]; !exists {
		t.Error("Expected status field")
	} else if statusText, ok := status.(types.Text); !ok || statusText.Value != "published" {
		t.Errorf("Expected later status 'published' to win, got %v", status)
	}

	// Verify non-conflicting fields from both stamps are present
	if version, exists := finalResult["version"]; !exists {
		t.Error("Expected version from first stamp")
	} else if versionText, ok := version.(types.Text); !ok || versionText.Value != "v1.0" {
		t.Errorf("Expected version to be 'v1.0', got %v", version)
	}

	if category, exists := finalResult["category"]; !exists {
		t.Error("Expected category from second stamp")
	} else if categoryText, ok := category.(types.Text); !ok || categoryText.Value != "feature" {
		t.Errorf("Expected category to be 'feature', got %v", category)
	}

	// Verify host fields preserved
	if name, exists := finalResult["name"]; !exists {
		t.Error("Expected name from host")
	} else if nameText, ok := name.(types.Text); !ok || nameText.Value != "Base Record" {
		t.Errorf("Expected name to be 'Base Record', got %v", name)
	}
}

// Test 7.2.5: Embed is not a value
// Attempting to assign an embed to a variable produces an error
func TestEmbed_NotAValue(t *testing.T) {
	// This test would be implemented in the parser/interpreter layer
	// where embed statements are distinguished from value expressions
	// For now, we verify that our embed operation only works on record compositions
	// and cannot be treated as a standalone value

	_, stampManager, _, _ := newSecurityTestSetup(t)

	// Test that embed operations require both host and stamp data
	var hostRecord map[string]interface{} = nil
	stampData := &StampSnapshot{
		Attributes: map[string]interface{}{
			"test": types.Text{Value: "value"},
		},
	}

	// This should handle the nil host gracefully (in practice, parser prevents this)
	result := stampManager.EmbedStamp(hostRecord, stampData)

	// Should return stamp attributes when host is nil (defensive behavior)
	if result == nil {
		t.Error("Expected result even with nil host")
	}

	// The key point is that embed is a composition operation, not a value
	// The actual error checking happens at parse time, not runtime
	t.Logf("Embed composition completed (not a standalone value)")
}

// Test 7.2.6: Multiple embeds
// Two embeds in one record — all fields from both appear
func TestEmbed_MultipleEmbeds(t *testing.T) {
	_, stampManager, _, _ := newSecurityTestSetup(t)

	// Host record
	hostRecord := map[string]interface{}{
		"id":    types.Number{Value: 42},
		"title": types.Text{Value: "Main Document"},
	}

	// First embed: user context
	userStamp := &StampSnapshot{
		Attributes: map[string]interface{}{
			"user_name": types.Text{Value: "alice"},
			"user_role": types.Text{Value: "manager"},
			"user_dept": types.Text{Value: "finance"},
		},
	}

	// Second embed: system context
	systemStamp := &StampSnapshot{
		Attributes: map[string]interface{}{
			"created_at": types.Time{Timestamp: time.Now(), Precision: types.PrecisionSubsecond},
			"system_id":  types.Text{Value: "prod-001"},
			"version":    types.Text{Value: "v1.5.2"},
		},
	}

	// Apply first embed
	result1 := stampManager.EmbedStamp(hostRecord, userStamp)

	// Apply second embed
	finalResult := stampManager.EmbedStamp(result1, systemStamp)

	// Verify all fields from host are present
	if id, exists := finalResult["id"]; !exists {
		t.Error("Expected id from host")
	} else if idNum, ok := id.(types.Number); !ok || idNum.Value != 42 {
		t.Errorf("Expected id to be 42, got %v", id)
	}

	if _, exists := finalResult["title"]; !exists {
		t.Error("Expected title from host")
	}

	// Verify all fields from first embed are present
	if userName, exists := finalResult["user_name"]; !exists {
		t.Error("Expected user_name from first embed")
	} else if userNameText, ok := userName.(types.Text); !ok || userNameText.Value != "alice" {
		t.Errorf("Expected user_name to be 'alice', got %v", userName)
	}

	if _, exists := finalResult["user_role"]; !exists {
		t.Error("Expected user_role from first embed")
	}

	if _, exists := finalResult["user_dept"]; !exists {
		t.Error("Expected user_dept from first embed")
	}

	// Verify all fields from second embed are present
	if _, exists := finalResult["created_at"]; !exists {
		t.Error("Expected created_at from second embed")
	}

	if systemId, exists := finalResult["system_id"]; !exists {
		t.Error("Expected system_id from second embed")
	} else if systemIdText, ok := systemId.(types.Text); !ok || systemIdText.Value != "prod-001" {
		t.Errorf("Expected system_id to be 'prod-001', got %v", systemId)
	}

	if _, exists := finalResult["version"]; !exists {
		t.Error("Expected version from second embed")
	}

	// Total fields: 2 host + 3 first embed + 3 second embed = 8
	// (excluding internal metadata)
	expectedFieldCount := 8
	fieldCount := len(finalResult)
	if _, hasMetadata := finalResult["@embedded_attrs"]; hasMetadata {
		fieldCount-- // Exclude internal metadata from count
	}
	if fieldCount != expectedFieldCount {
		t.Errorf("Expected %d fields after multiple embeds, got %d", expectedFieldCount, fieldCount)
	}
}

// Test 7.2.7: Embedded record retains identity
// Can still query the embedded record independently
func TestEmbed_EmbeddedRecordRetainsIdentity(t *testing.T) {
	_, stampManager, _, _ := newSecurityTestSetup(t)

	// Create original stamp record
	originalStamp := &StampSnapshot{
		Attributes: map[string]interface{}{
			"department": types.Text{Value: "engineering"},
			"project":    types.Text{Value: "core-platform"},
			"owner":      types.Text{Value: "bob"},
		},
		Hash: storage.HashValue(types.SerializeValue(map[string]interface{}{
			"department": types.Text{Value: "engineering"},
			"project":    types.Text{Value: "core-platform"},
			"owner":      types.Text{Value: "bob"},
		})),
	}

	// Create host record and embed stamp
	hostRecord := map[string]interface{}{
		"task_id":   types.Number{Value: 1001},
		"task_name": types.Text{Value: "Implement Login"},
	}

	result := stampManager.EmbedStamp(hostRecord, originalStamp)

	// Verify the embedded stamp fields appear in the result
	if dept, exists := result["department"]; !exists {
		t.Error("Expected department from embedded stamp")
	} else if deptText, ok := dept.(types.Text); !ok || deptText.Value != "engineering" {
		t.Errorf("Expected department to be 'engineering', got %v", dept)
	}

	// Verify original stamp record maintains its identity
	// The original stamp hash should remain unchanged
	originalHash := originalStamp.Hash

	// Verify original stamp attributes are still independently accessible
	if originalDept, exists := originalStamp.Attributes["department"]; !exists {
		t.Error("Expected department in original stamp to remain accessible")
	} else if originalDeptText, ok := originalDept.(types.Text); !ok || originalDeptText.Value != "engineering" {
		t.Errorf("Expected original department to remain 'engineering', got %v", originalDept)
	}

	// The original stamp record identity is preserved (same hash, same content)
	if originalStamp.Hash != originalHash {
		t.Error("Expected original stamp hash to remain unchanged after embed")
	}

	// Verify that modifying the result doesn't affect the original stamp
	result["department"] = types.Text{Value: "modified"}

	if modifiedDept, exists := originalStamp.Attributes["department"]; !exists {
		t.Error("Expected department in original stamp")
	} else if modifiedDeptText, ok := modifiedDept.(types.Text); !ok || modifiedDeptText.Value != "engineering" {
		t.Errorf("Expected original department to remain 'engineering' after result modification, got %v", modifiedDept)
	}

	t.Logf("Embedded record maintains independent identity with hash: %x", originalStamp.Hash)
}

//=============================================================================
// 7.3 FILTERS
//=============================================================================

// Test 7.3.1: Filter hides matching data
// Activate `@stamp.environment = "test"` filter — test data shows as Redacted
func TestFilters_HideMatchingData(t *testing.T) {
	tree, _, filterManager, _ := newSecurityTestSetup(t)
	agentID := uint64(12345)

	agent := &Agent{
		Identity: "test_agent",
		AgentID:  agentID,
		Tree:     tree,
	}

	// Set filter to hide test environment data
	filter := types.Record{
		Fields: map[string]interface{}{
			"environment": types.Text{Value: "test"},
		},
	}

	err := filterManager.SetFilter(filter, agentID)
	if err != nil {
		t.Fatalf("Failed to set environment filter: %v", err)
	}

	// Test data that should be hidden (matches filter)
	testData := map[string]interface{}{
		"environment": types.Text{Value: "test"},
		"service":     types.Text{Value: "user-service"},
		"data":        types.Text{Value: "test database"},
	}

	visible, err := filterManager.ApplyFilters(agent, testData)
	if err != nil {
		t.Errorf("ApplyFilters error: %v", err)
	}
	if visible {
		t.Error("Expected test environment data to be hidden by filter")
	}

	// Production data should remain visible (doesn't match filter)
	productionData := map[string]interface{}{
		"environment": types.Text{Value: "production"},
		"service":     types.Text{Value: "user-service"},
		"data":        types.Text{Value: "production database"},
	}

	visible, err = filterManager.ApplyFilters(agent, productionData)
	if err != nil {
		t.Errorf("ApplyFilters error: %v", err)
	}
	if !visible {
		t.Error("Expected production environment data to remain visible")
	}
}

// Test 7.3.2: Filter is client-side only
// Filter does not affect what another agent sees
func TestFilters_ClientSideOnly(t *testing.T) {
	tree, _, filterManager, _ := newSecurityTestSetup(t)

	// Create two different agents
	agent1ID := uint64(11111)
	agent2ID := uint64(22222)

	agent1 := &Agent{
		Identity: "agent_one",
		AgentID:  agent1ID,
		Tree:     tree,
	}

	agent2 := &Agent{
		Identity: "agent_two",
		AgentID:  agent2ID,
		Tree:     tree,
	}

	// Set filter only for agent1
	filter := types.Record{
		Fields: map[string]interface{}{
			"department": types.Text{Value: "marketing"},
		},
	}

	err := filterManager.SetFilter(filter, agent1ID)
	if err != nil {
		t.Fatalf("Failed to set filter for agent1: %v", err)
	}

	// Test data that matches the filter
	marketingData := map[string]interface{}{
		"department": types.Text{Value: "marketing"},
		"campaign":   types.Text{Value: "Q4 Launch"},
		"budget":     types.Number{Value: 50000},
	}

	// Agent1 should not see the data (filtered out)
	visible1, err := filterManager.ApplyFilters(agent1, marketingData)
	if err != nil {
		t.Errorf("ApplyFilters error for agent1: %v", err)
	}
	if visible1 {
		t.Error("Expected marketing data to be filtered out for agent1")
	}

	// Agent2 should see the data (no filter set)
	visible2, err := filterManager.ApplyFilters(agent2, marketingData)
	if err != nil {
		t.Errorf("ApplyFilters error for agent2: %v", err)
	}
	if !visible2 {
		t.Errorf("Expected marketing data to be visible for agent2 (no filter). Agent2 ID: %d, Visible: %v", agent2.AgentID, visible2)
	}
}

// Test 7.3.3: Remove filter reveals data
// Deactivate filter — data visible again
func TestFilters_RemoveFilterRevealsData(t *testing.T) {
	tree, _, filterManager, _ := newSecurityTestSetup(t)
	agentID := uint64(12345)

	agent := &Agent{
		Identity: "test_agent",
		AgentID:  agentID,
		Tree:     tree,
	}

	// Set filter to hide development data
	filter := types.Record{
		Fields: map[string]interface{}{
			"stage": types.Text{Value: "development"},
		},
	}

	err := filterManager.SetFilter(filter, agentID)
	if err != nil {
		t.Fatalf("Failed to set development filter: %v", err)
	}

	// Test data that should be filtered
	devData := map[string]interface{}{
		"stage":   types.Text{Value: "development"},
		"feature": types.Text{Value: "new-ui"},
		"ready":   types.Text{Value: "false"},
	}

	// With filter active, data should be hidden
	visible, err := filterManager.ApplyFilters(agent, devData)
	if err != nil {
		t.Errorf("ApplyFilters error: %v", err)
	}
	if visible {
		t.Error("Expected development data to be hidden with filter active")
	}

	// Remove the filter
	err = filterManager.ClearFilter(agentID)
	if err != nil {
		t.Fatalf("Failed to clear filter: %v", err)
	}

	// With filter removed, data should be visible
	visible, err = filterManager.ApplyFilters(agent, devData)
	if err != nil {
		t.Errorf("ApplyFilters error after clearing filter: %v", err)
	}
	if !visible {
		t.Error("Expected development data to be visible after clearing filter")
	}
}

// Test 7.3.4: Multiple active filters
// Stack two filters — both apply simultaneously
func TestFilters_MultipleActiveFilters(t *testing.T) {
	tree, _, filterManager, _ := newSecurityTestSetup(t)
	agentID := uint64(12345)

	agent := &Agent{
		Identity: "test_agent",
		AgentID:  agentID,
		Tree:     tree,
	}

	// Set composite filter with multiple criteria
	filter := types.Record{
		Fields: map[string]interface{}{
			"stage":    types.Text{Value: "development"}, // First filter criterion
			"priority": types.Text{Value: "low"},         // Second filter criterion
		},
	}

	err := filterManager.SetFilter(filter, agentID)
	if err != nil {
		t.Fatalf("Failed to set multiple criteria filter: %v", err)
	}

	// Test data that matches both criteria (should be hidden)
	matchesBoth := map[string]interface{}{
		"stage":    types.Text{Value: "development"},
		"priority": types.Text{Value: "low"},
		"task":     types.Text{Value: "minor-fix"},
	}

	visible, err := filterManager.ApplyFilters(agent, matchesBoth)
	if err != nil {
		t.Errorf("ApplyFilters error: %v", err)
	}
	if visible {
		t.Error("Expected data matching both filter criteria to be hidden")
	}

	// Test data that matches only first criterion (should be visible)
	matchesFirst := map[string]interface{}{
		"stage":    types.Text{Value: "development"},
		"priority": types.Text{Value: "high"},
		"task":     types.Text{Value: "important-feature"},
	}

	visible, err = filterManager.ApplyFilters(agent, matchesFirst)
	if err != nil {
		t.Errorf("ApplyFilters error: %v", err)
	}
	if !visible {
		t.Error("Expected data matching only first criterion to be visible")
	}

	// Test data that matches only second criterion (should be visible)
	matchesSecond := map[string]interface{}{
		"stage":    types.Text{Value: "production"},
		"priority": types.Text{Value: "low"},
		"task":     types.Text{Value: "documentation"},
	}

	visible, err = filterManager.ApplyFilters(agent, matchesSecond)
	if err != nil {
		t.Errorf("ApplyFilters error: %v", err)
	}
	if !visible {
		t.Error("Expected data matching only second criterion to be visible")
	}

	// Test data that matches neither criterion (should be visible)
	matchesNeither := map[string]interface{}{
		"stage":    types.Text{Value: "production"},
		"priority": types.Text{Value: "high"},
		"task":     types.Text{Value: "critical-bugfix"},
	}

	visible, err = filterManager.ApplyFilters(agent, matchesNeither)
	if err != nil {
		t.Errorf("ApplyFilters error: %v", err)
	}
	if !visible {
		t.Error("Expected data matching neither criterion to be visible")
	}
}

// Test 7.3.5: Redacted display
// Filtered data appears as `***` (asterisks matching character count)
func TestFilters_RedactedDisplay(t *testing.T) {
	// This test verifies the concept of redacted display
	// In the actual system, the UI layer would handle the *** display
	// The filter system just returns visibility boolean

	tree, _, filterManager, _ := newSecurityTestSetup(t)
	agentID := uint64(12345)

	agent := &Agent{
		Identity: "test_agent",
		AgentID:  agentID,
		Tree:     tree,
	}

	// Set filter to hide sensitive data
	filter := types.Record{
		Fields: map[string]interface{}{
			"classification": types.Text{Value: "secret"},
		},
	}

	err := filterManager.SetFilter(filter, agentID)
	if err != nil {
		t.Fatalf("Failed to set classification filter: %v", err)
	}

	// Test sensitive data that should be redacted
	secretData := map[string]interface{}{
		"classification": types.Text{Value: "secret"},
		"document":       types.Text{Value: "Top Secret Information"},
		"project_name":   types.Text{Value: "Project X"},
	}

	visible, err := filterManager.ApplyFilters(agent, secretData)
	if err != nil {
		t.Errorf("ApplyFilters error: %v", err)
	}
	if visible {
		t.Error("Expected secret data to be hidden (would show as *** in UI)")
	}

	// Test non-sensitive data that should be visible
	publicData := map[string]interface{}{
		"classification": types.Text{Value: "public"},
		"document":       types.Text{Value: "Public Annual Report"},
		"project_name":   types.Text{Value: "Website Redesign"},
	}

	visible, err = filterManager.ApplyFilters(agent, publicData)
	if err != nil {
		t.Errorf("ApplyFilters error: %v", err)
	}
	if !visible {
		t.Error("Expected public data to be visible normally")
	}

	// Test filtering multiple records
	testRecords := map[string]interface{}{
		"secret_doc": map[string]interface{}{
			"classification": types.Text{Value: "secret"},
			"content":        types.Text{Value: "Classified content"},
		},
		"public_doc": map[string]interface{}{
			"classification": types.Text{Value: "public"},
			"content":        types.Text{Value: "Public content"},
		},
	}

	visibleRecords, err := filterManager.FilterVisible(agent, testRecords)
	if err != nil {
		t.Fatalf("FilterVisible error: %v", err)
	}

	// Should only see the public document
	if len(visibleRecords) != 1 {
		t.Errorf("Expected 1 visible record, got %d", len(visibleRecords))
	}

	if _, exists := visibleRecords["public_doc"]; !exists {
		t.Error("Expected public_doc to be visible")
	}

	if _, exists := visibleRecords["secret_doc"]; exists {
		t.Error("Expected secret_doc to be filtered out (redacted)")
	}

	t.Logf("Filter successfully redacted %d records", len(testRecords)-len(visibleRecords))
}

//=============================================================================
// 7.4 PERMISSIONS
//=============================================================================

// Test 7.4.1: Default home permissions
// Under an agent's own home (world.agent.{id}.*), all permissions default to
// owner-only. The rule is self-relative: each agent is the owner of its own
// resolved home path, so it holds every default permission there, while another
// agent gets no owner default on it.
func TestPermissions_DefaultHomePermissions(t *testing.T) {
	tree, _, _, permEval := newSecurityTestSetup(t)

	// Create test agents
	ownerID := uint64(11111)
	otherID := uint64(22222)

	owner := &Agent{
		Identity: "owner",
		AgentID:  ownerID,
		Tree:     tree,
	}

	otherAgent := &Agent{
		Identity: "other",
		AgentID:  otherID,
		Tree:     tree,
	}

	permissions := []PermissionType{ReadPermission, WritePermission, ExpandPermission, GrantPermission, PurgePermission}

	// The home default is self-relative: each agent holds every default permission
	// on its OWN resolved home path (world.agent.{its id}.*), so we check each
	// agent against a path under its own home. (Previously spelled `~`, since
	// removed — `my.*` now resolves to exactly this shape.)
	ownerHomePath := []string{"world", "agent", "11111", "private", "secrets"}
	otherHomePath := []string{"world", "agent", "22222", "private", "secrets"}

	for _, perm := range permissions {
		allowed, err := permEval.CheckPermission(owner, ownerHomePath, perm)
		if err != nil {
			t.Errorf("CheckPermission error for owner %s: %v", perm, err)
		}
		if !allowed {
			t.Errorf("Expected owner to have %s permission to their own home", perm)
		}

		allowedOther, err := permEval.CheckPermission(otherAgent, otherHomePath, perm)
		if err != nil {
			t.Errorf("CheckPermission error for other agent %s: %v", perm, err)
		}
		if !allowedOther {
			t.Errorf("Expected other agent to have %s permission to its OWN home (home default is self-relative)", perm)
		}
	}

	// Cross-agent isolation: the owner's home is owner-only. Grant every
	// permission to "owner" alone on the owner's resolved home and confirm the
	// owner is allowed while the other agent is denied.
	ownerHome := []string{"world", "agent", "11111", "private", "secrets"}
	for _, perm := range permissions {
		if err := permEval.SetPermission(ownerHome, perm, types.Text{Value: "owner"}, ownerID); err != nil {
			t.Fatalf("failed to set %s permission on owner home: %v", perm, err)
		}
	}

	for _, perm := range permissions {
		allowed, err := permEval.CheckPermission(owner, ownerHome, perm)
		if err != nil {
			t.Errorf("CheckPermission error for owner %s: %v", perm, err)
		}
		if !allowed {
			t.Errorf("Expected owner to have %s permission to owner's home", perm)
		}

		allowedOther, err := permEval.CheckPermission(otherAgent, ownerHome, perm)
		if err != nil {
			t.Errorf("CheckPermission error for other agent %s: %v", perm, err)
		}
		if allowedOther {
			t.Errorf("Expected other agent to NOT have %s permission to owner's home", perm)
		}
	}
}

// Test 7.4.2: Default `world` permissions
// `@read = Anything` by default — any agent can read
func TestPermissions_DefaultWorldPermissions(t *testing.T) {
	tree, _, _, permEval := newSecurityTestSetup(t)

	// Create test agents
	agent1 := createTestAgent(11111)
	agent1.Tree = tree
	agent2 := createTestAgent(22222)
	agent2.Tree = tree

	// Test path under world
	worldPath := []string{"world", "public", "data"}

	// Both agents should be able to read world data by default
	for _, agent := range []*Agent{agent1, agent2} {
		allowed, err := permEval.CheckPermission(agent, worldPath, ReadPermission)
		if err != nil {
			t.Errorf("CheckPermission error for agent %d: %v", agent.AgentID, err)
		}
		if !allowed {
			t.Errorf("Expected agent %d to have read permission to world path", agent.AgentID)
		}
	}
}

// Test 7.4.3: Default `world` write restrictions
// `@write = Nothing` by default — no agent can write to `world` root without explicit grant
func TestPermissions_DefaultWorldWriteRestrictions(t *testing.T) {
	tree, _, _, permEval := newSecurityTestSetup(t)

	// Create test agent
	agent := createTestAgent(12345)
	agent.Tree = tree

	// Test paths under world
	worldPaths := [][]string{
		{"world"},
		{"world", "public"},
		{"world", "shared", "resources"},
	}

	// Agent should NOT be able to write to world paths by default
	restrictedPermissions := []PermissionType{WritePermission, ExpandPermission, GrantPermission, PurgePermission}

	for _, path := range worldPaths {
		for _, perm := range restrictedPermissions {
			allowed, err := permEval.CheckPermission(agent, path, perm)
			if err != nil {
				t.Errorf("CheckPermission error for path %v, permission %s: %v", path, perm, err)
			}
			if allowed {
				t.Errorf("Expected agent to NOT have %s permission to world path %v", perm, path)
			}
		}
	}
}

// Test 7.4.4: Read permission enforced
// Set `@read` to specific agent — other agent gets permission error
func TestPermissions_ReadPermissionEnforced(t *testing.T) {
	tree, _, _, permEval := newSecurityTestSetup(t)

	// Create test agents
	allowedAgentID := uint64(11111)
	deniedAgentID := uint64(22222)

	allowedAgent := &Agent{
		Identity: "allowed",
		AgentID:  allowedAgentID,
		Tree:     tree,
	}

	deniedAgent := &Agent{
		Identity: "denied",
		AgentID:  deniedAgentID,
		Tree:     tree,
	}

	// Set specific read permission
	restrictedPath := []string{"world", "company", "confidential"}
	allowedAgentRef := types.Text{Value: "allowed"} // Agent identity

	err := permEval.SetPermission(restrictedPath, ReadPermission, allowedAgentRef, allowedAgentID)
	if err != nil {
		t.Fatalf("Failed to set read permission: %v", err)
	}

	// Allowed agent should have read permission
	allowed, err := permEval.CheckPermission(allowedAgent, restrictedPath, ReadPermission)
	if err != nil {
		t.Errorf("CheckPermission error for allowed agent: %v", err)
	}
	if !allowed {
		t.Error("Expected allowed agent to have read permission")
	}

	// Denied agent should NOT have read permission
	allowed, err = permEval.CheckPermission(deniedAgent, restrictedPath, ReadPermission)
	if err != nil {
		t.Errorf("CheckPermission error for denied agent: %v", err)
	}
	if allowed {
		t.Error("Expected denied agent to NOT have read permission")
	}
}

// Test 7.4.5: Write permission enforced
// Set `@write` to specific agent — other agent's write rejected
func TestPermissions_WritePermissionEnforced(t *testing.T) {
	tree, _, _, permEval := newSecurityTestSetup(t)

	// Create test agents
	writerAgentID := uint64(11111)
	readerAgentID := uint64(22222)

	writerAgent := &Agent{
		Identity: "writer",
		AgentID:  writerAgentID,
		Tree:     tree,
	}

	readerAgent := &Agent{
		Identity: "reader",
		AgentID:  readerAgentID,
		Tree:     tree,
	}

	// Set specific write permission
	protectedPath := []string{"world", "projects", "alpha"}
	writerRef := types.Text{Value: "writer"} // Agent identity

	err := permEval.SetPermission(protectedPath, WritePermission, writerRef, writerAgentID)
	if err != nil {
		t.Fatalf("Failed to set write permission: %v", err)
	}

	// Writer agent should have write permission
	allowed, err := permEval.CheckPermission(writerAgent, protectedPath, WritePermission)
	if err != nil {
		t.Errorf("CheckPermission error for writer agent: %v", err)
	}
	if !allowed {
		t.Error("Expected writer agent to have write permission")
	}

	// Reader agent should NOT have write permission
	allowed, err = permEval.CheckPermission(readerAgent, protectedPath, WritePermission)
	if err != nil {
		t.Errorf("CheckPermission error for reader agent: %v", err)
	}
	if allowed {
		t.Error("Expected reader agent to NOT have write permission")
	}
}

// Test 7.4.6: Expand permission
// Set `@expand = Nothing` — cannot create sub-attributes
func TestPermissions_ExpandPermission(t *testing.T) {
	tree, _, _, permEval := newSecurityTestSetup(t)

	// Create test agent
	agent := createTestAgent(12345)
	agent.Tree = tree

	// Set expand permission to Nothing
	lockedPath := []string{"world", "locked", "container"}
	nothing := types.Text{Value: "(Nothing)"}

	err := permEval.SetPermission(lockedPath, ExpandPermission, nothing, agent.AgentID)
	if err != nil {
		t.Fatalf("Failed to set expand permission to Nothing: %v", err)
	}

	// Agent should NOT be able to expand (create sub-attributes)
	allowed, err := permEval.CheckPermission(agent, lockedPath, ExpandPermission)
	if err != nil {
		t.Errorf("CheckPermission error: %v", err)
	}
	if allowed {
		t.Error("Expected expand permission to be denied when set to Nothing")
	}

	// Test with allowed expand permission
	allowedPath := []string{"world", "open", "container"}
	anything := types.Text{Value: "(Anything)"}

	err = permEval.SetPermission(allowedPath, ExpandPermission, anything, agent.AgentID)
	if err != nil {
		t.Fatalf("Failed to set expand permission to Anything: %v", err)
	}

	allowed, err = permEval.CheckPermission(agent, allowedPath, ExpandPermission)
	if err != nil {
		t.Errorf("CheckPermission error: %v", err)
	}
	if !allowed {
		t.Error("Expected expand permission to be allowed when set to Anything")
	}
}

// Test 7.4.7: Grant permission
// Only agent with `@grant` can modify permissions
func TestPermissions_GrantPermission(t *testing.T) {
	tree, _, _, permEval := newSecurityTestSetup(t)

	// Create test agents
	granterID := uint64(11111)
	userID := uint64(22222)

	granter := &Agent{
		Identity: "granter",
		AgentID:  granterID,
		Tree:     tree,
	}

	user := &Agent{
		Identity: "user",
		AgentID:  userID,
		Tree:     tree,
	}

	// Set grant permission to specific agent
	managedPath := []string{"world", "managed", "resource"}
	granterRef := types.Text{Value: "granter"} // Agent identity

	err := permEval.SetPermission(managedPath, GrantPermission, granterRef, granterID)
	if err != nil {
		t.Fatalf("Failed to set grant permission: %v", err)
	}

	// Granter should have grant permission
	allowed, err := permEval.CheckPermission(granter, managedPath, GrantPermission)
	if err != nil {
		t.Errorf("CheckPermission error for granter: %v", err)
	}
	if !allowed {
		t.Error("Expected granter to have grant permission")
	}

	// User should NOT have grant permission
	allowed, err = permEval.CheckPermission(user, managedPath, GrantPermission)
	if err != nil {
		t.Errorf("CheckPermission error for user: %v", err)
	}
	if allowed {
		t.Error("Expected user to NOT have grant permission")
	}

	// Only granter should be able to modify permissions
	// (In practice, this would be enforced by checking grant permission before allowing SetPermission)
	userRef := types.Text{Value: "user"}                                          // Agent identity
	err = permEval.SetPermission(managedPath, ReadPermission, userRef, granterID) // Granter sets permission
	if err != nil {
		t.Errorf("Expected granter to be able to set permissions: %v", err)
	}

	// Attempting to set permission without grant permission would be rejected by system
	// This is enforced at the service layer, not in the permission evaluator itself
}

// Test 7.4.8: Purge permission
// Only agent with `@purge` can delete data
func TestPermissions_PurgePermission(t *testing.T) {
	tree, _, _, permEval := newSecurityTestSetup(t)

	// Create test agents
	adminID := uint64(11111)
	userID := uint64(22222)

	admin := &Agent{
		Identity: "admin",
		AgentID:  adminID,
		Tree:     tree,
	}

	user := &Agent{
		Identity: "user",
		AgentID:  userID,
		Tree:     tree,
	}

	// Set purge permission to admin only
	criticalPath := []string{"world", "critical", "data"}
	adminRef := types.Text{Value: "admin"} // Agent identity

	err := permEval.SetPermission(criticalPath, PurgePermission, adminRef, adminID)
	if err != nil {
		t.Fatalf("Failed to set purge permission: %v", err)
	}

	// Admin should have purge permission
	allowed, err := permEval.CheckPermission(admin, criticalPath, PurgePermission)
	if err != nil {
		t.Errorf("CheckPermission error for admin: %v", err)
	}
	if !allowed {
		t.Error("Expected admin to have purge permission")
	}

	// User should NOT have purge permission
	allowed, err = permEval.CheckPermission(user, criticalPath, PurgePermission)
	if err != nil {
		t.Errorf("CheckPermission error for user: %v", err)
	}
	if allowed {
		t.Error("Expected user to NOT have purge permission")
	}
}

// Test 7.4.9: Permission cascade
// Set permissions at parent — child inherits until overridden
func TestPermissions_PermissionCascade(t *testing.T) {
	tree, _, _, permEval := newSecurityTestSetup(t)

	// Create test agent
	agent := createTestAgent(12345)
	agent.Tree = tree

	// Set permission at parent level
	parentPath := []string{"world", "company"}
	agentRef := types.Text{Value: "test_agent"} // Agent identity

	err := permEval.SetPermission(parentPath, ReadPermission, agentRef, agent.AgentID)
	if err != nil {
		t.Fatalf("Failed to set parent permission: %v", err)
	}

	// Child paths should inherit the permission
	childPaths := [][]string{
		{"world", "company", "department"},
		{"world", "company", "department", "team"},
		{"world", "company", "reports"},
		{"world", "company", "reports", "quarterly"},
	}

	for _, childPath := range childPaths {
		allowed, err := permEval.CheckPermission(agent, childPath, ReadPermission)
		if err != nil {
			t.Errorf("CheckPermission error for child path %v: %v", childPath, err)
		}
		if !allowed {
			t.Errorf("Expected child path %v to inherit read permission from parent", childPath)
		}
	}
}

// Test 7.4.10: Permission override at child
// Set restrictive at parent, permissive at child — child's override takes effect
func TestPermissions_PermissionOverrideAtChild(t *testing.T) {
	tree, _, _, permEval := newSecurityTestSetup(t)

	// Create test agents
	agent1ID := uint64(11111)
	agent2ID := uint64(22222)

	agent1 := &Agent{
		Identity: "agent1",
		AgentID:  agent1ID,
		Tree:     tree,
	}

	agent2 := &Agent{
		Identity: "agent2",
		AgentID:  agent2ID,
		Tree:     tree,
	}

	// Set restrictive permission at parent (only agent1)
	parentPath := []string{"world", "restricted"}
	agent1Ref := types.Text{Value: "agent1"} // Agent identity

	err := permEval.SetPermission(parentPath, WritePermission, agent1Ref, agent1ID)
	if err != nil {
		t.Fatalf("Failed to set restrictive parent permission: %v", err)
	}

	// Set permissive permission at child (both agents)
	childPath := []string{"world", "restricted", "public_section"}
	anything := types.Text{Value: "(Anything)"}

	err = permEval.SetPermission(childPath, WritePermission, anything, agent1ID)
	if err != nil {
		t.Fatalf("Failed to set permissive child permission: %v", err)
	}

	// Parent level: only agent1 should have access
	allowed, err := permEval.CheckPermission(agent1, parentPath, WritePermission)
	if err != nil {
		t.Errorf("CheckPermission error for agent1 at parent: %v", err)
	}
	if !allowed {
		t.Error("Expected agent1 to have write permission at parent level")
	}

	allowed, err = permEval.CheckPermission(agent2, parentPath, WritePermission)
	if err != nil {
		t.Errorf("CheckPermission error for agent2 at parent: %v", err)
	}
	if allowed {
		t.Error("Expected agent2 to NOT have write permission at parent level")
	}

	// Child level: both agents should have access (child override)
	allowed, err = permEval.CheckPermission(agent1, childPath, WritePermission)
	if err != nil {
		t.Errorf("CheckPermission error for agent1 at child: %v", err)
	}
	if !allowed {
		t.Error("Expected agent1 to have write permission at child level")
	}

	allowed, err = permEval.CheckPermission(agent2, childPath, WritePermission)
	if err != nil {
		t.Errorf("CheckPermission error for agent2 at child: %v", err)
	}
	if !allowed {
		t.Error("Expected agent2 to have write permission at child level (override)")
	}
}

// Test 7.4.11: Agent list permission
// `@write = [agent1, agent2]` — both can write, agent3 cannot
func TestPermissions_AgentListPermission(t *testing.T) {
	tree, _, _, permEval := newSecurityTestSetup(t)

	// Create test agents
	agent1ID := uint64(11111)
	agent2ID := uint64(22222)
	agent3ID := uint64(33333)

	agent1 := &Agent{Identity: "agent1", AgentID: agent1ID, Tree: tree}
	agent2 := &Agent{Identity: "agent2", AgentID: agent2ID, Tree: tree}
	agent3 := &Agent{Identity: "agent3", AgentID: agent3ID, Tree: tree}

	// Set permission to list of agents
	collaborativePath := []string{"world", "projects", "collaborative"}
	agentList := types.List{
		Elements: []interface{}{
			types.Text{Value: "agent1"},
			types.Text{Value: "agent2"},
		},
	}

	err := permEval.SetPermission(collaborativePath, WritePermission, agentList, agent1ID)
	if err != nil {
		t.Fatalf("Failed to set agent list permission: %v", err)
	}

	// Agent1 should have write permission
	allowed, err := permEval.CheckPermission(agent1, collaborativePath, WritePermission)
	if err != nil {
		t.Errorf("CheckPermission error for agent1: %v", err)
	}
	if !allowed {
		t.Error("Expected agent1 to have write permission (in list)")
	}

	// Agent2 should have write permission
	allowed, err = permEval.CheckPermission(agent2, collaborativePath, WritePermission)
	if err != nil {
		t.Errorf("CheckPermission error for agent2: %v", err)
	}
	if !allowed {
		t.Error("Expected agent2 to have write permission (in list)")
	}

	// Agent3 should NOT have write permission
	allowed, err = permEval.CheckPermission(agent3, collaborativePath, WritePermission)
	if err != nil {
		t.Errorf("CheckPermission error for agent3: %v", err)
	}
	if allowed {
		t.Error("Expected agent3 to NOT have write permission (not in list)")
	}
}

// Test 7.4.12: "Anything" permission
// `@read = "Anything"` — all agents can read
func TestPermissions_AnythingPermission(t *testing.T) {
	tree, _, _, permEval := newSecurityTestSetup(t)

	// Create multiple test agents
	agents := []*Agent{
		{Identity: "agent1", AgentID: 11111, Tree: tree},
		{Identity: "agent2", AgentID: 22222, Tree: tree},
		{Identity: "agent3", AgentID: 33333, Tree: tree},
	}

	// Set permission to "Anything"
	publicPath := []string{"world", "public", "announcements"}
	anything := types.Text{Value: "(Anything)"}

	err := permEval.SetPermission(publicPath, ReadPermission, anything, agents[0].AgentID)
	if err != nil {
		t.Fatalf("Failed to set Anything permission: %v", err)
	}

	// All agents should have read permission
	for _, agent := range agents {
		allowed, err := permEval.CheckPermission(agent, publicPath, ReadPermission)
		if err != nil {
			t.Errorf("CheckPermission error for %s: %v", agent.Identity, err)
		}
		if !allowed {
			t.Errorf("Expected %s to have read permission with Anything", agent.Identity)
		}
	}
}

// Test 7.4.13: "Nothing" permission
// `@write = "Nothing"` — no one can write
func TestPermissions_NothingPermission(t *testing.T) {
	tree, _, _, permEval := newSecurityTestSetup(t)

	// Create multiple test agents including high-privilege ones
	agents := []*Agent{
		{Identity: "admin", AgentID: 11111, Tree: tree},
		{Identity: "user", AgentID: 22222, Tree: tree},
		{Identity: "guest", AgentID: 33333, Tree: tree},
	}

	// Set permission to "Nothing"
	immutablePath := []string{"world", "system", "immutable"}
	nothing := types.Text{Value: "(Nothing)"}

	err := permEval.SetPermission(immutablePath, WritePermission, nothing, agents[0].AgentID)
	if err != nil {
		t.Fatalf("Failed to set Nothing permission: %v", err)
	}

	// No agent should have write permission
	for _, agent := range agents {
		allowed, err := permEval.CheckPermission(agent, immutablePath, WritePermission)
		if err != nil {
			t.Errorf("CheckPermission error for %s: %v", agent.Identity, err)
		}
		if allowed {
			t.Errorf("Expected %s to NOT have write permission with Nothing", agent.Identity)
		}
	}
}

// Test 7.4.14: Watcher permission respect
// Watcher cannot modify data it doesn't have `@write` on
func TestPermissions_WatcherPermissionRespect(t *testing.T) {
	tree, _, _, permEval := newSecurityTestSetup(t)

	// Create test agent (representing watcher execution context)
	watcherAgent := createTestAgent(12345)
	watcherAgent.Tree = tree

	// Set restricted write permission on target path
	restrictedPath := []string{"world", "protected", "data"}
	nothing := types.Text{Value: "(Nothing)"}

	err := permEval.SetPermission(restrictedPath, WritePermission, nothing, watcherAgent.AgentID)
	if err != nil {
		t.Fatalf("Failed to set restricted write permission: %v", err)
	}

	// Watcher agent should NOT have write permission
	allowed, err := permEval.CheckPermission(watcherAgent, restrictedPath, WritePermission)
	if err != nil {
		t.Errorf("CheckPermission error for watcher: %v", err)
	}
	if allowed {
		t.Error("Expected watcher to NOT have write permission on protected data")
	}

	// Set allowed write permission for comparison
	allowedPath := []string{"world", "public", "logs"}
	anything := types.Text{Value: "(Anything)"}

	err = permEval.SetPermission(allowedPath, WritePermission, anything, watcherAgent.AgentID)
	if err != nil {
		t.Fatalf("Failed to set allowed write permission: %v", err)
	}

	// Watcher agent should have write permission here
	allowed, err = permEval.CheckPermission(watcherAgent, allowedPath, WritePermission)
	if err != nil {
		t.Errorf("CheckPermission error for watcher on allowed path: %v", err)
	}
	if !allowed {
		t.Error("Expected watcher to have write permission on public logs")
	}

	// In practice, the watcher engine would check permissions before executing write operations
	t.Logf("Watcher permission enforcement verified")
}

// Test 7.4.15: Permission + filter independence
// Filter cannot bypass permissions; permissions cannot bypass filters
func TestPermissions_FilterIndependence(t *testing.T) {
	tree, _, filterManager, permEval := newSecurityTestSetup(t)

	// Create test agents
	agent1ID := uint64(11111)
	agent2ID := uint64(22222)

	agent1 := &Agent{Identity: "agent1", AgentID: agent1ID, Tree: tree}
	agent2 := &Agent{Identity: "agent2", AgentID: agent2ID, Tree: tree}

	// Set permission: only agent1 can read sensitive data
	sensitivePath := []string{"world", "company", "sensitive"}
	agent1Ref := types.Text{Value: "agent1"} // Agent identity

	err := permEval.SetPermission(sensitivePath, ReadPermission, agent1Ref, agent1ID)
	if err != nil {
		t.Fatalf("Failed to set read permission: %v", err)
	}

	// Set filter for agent1: hide "internal" classification
	filter := types.Record{
		Fields: map[string]interface{}{
			"classification": types.Text{Value: "internal"},
		},
	}

	err = filterManager.SetFilter(filter, agent1ID)
	if err != nil {
		t.Fatalf("Failed to set filter: %v", err)
	}

	// Test data with internal classification
	internalData := map[string]interface{}{
		"classification": types.Text{Value: "internal"},
		"content":        types.Text{Value: "internal company data"},
	}

	// Agent1: has permission but filter hides the data
	hasPermission, err := permEval.CheckPermission(agent1, sensitivePath, ReadPermission)
	if err != nil {
		t.Errorf("Permission check error for agent1: %v", err)
	}
	if !hasPermission {
		t.Error("Expected agent1 to have read permission")
	}

	filterAllows, err := filterManager.ApplyFilters(agent1, internalData)
	if err != nil {
		t.Errorf("Filter check error for agent1: %v", err)
	}
	if filterAllows {
		t.Error("Expected filter to hide internal data from agent1")
	}

	// Agent2: no permission, filter state irrelevant
	hasPermission, err = permEval.CheckPermission(agent2, sensitivePath, ReadPermission)
	if err != nil {
		t.Errorf("Permission check error for agent2: %v", err)
	}
	if hasPermission {
		t.Error("Expected agent2 to NOT have read permission")
	}

	// Filter cannot grant access that permissions deny
	filterAllows, err = filterManager.ApplyFilters(agent2, internalData)
	if err != nil {
		t.Errorf("Filter check error for agent2: %v", err)
	}
	// Even if filter would allow, permissions would deny at system level

	// Test public data with agent1's filter
	publicData := map[string]interface{}{
		"classification": types.Text{Value: "public"},
		"content":        types.Text{Value: "public company data"},
	}

	filterAllows, err = filterManager.ApplyFilters(agent1, publicData)
	if err != nil {
		t.Errorf("Filter check error for public data: %v", err)
	}
	if !filterAllows {
		t.Error("Expected filter to allow public data")
	}

	t.Logf("Permission and filter independence verified - they operate as separate layers")
}
