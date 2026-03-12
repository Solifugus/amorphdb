package security

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

func TestFilterManager_SetAndGetFilter(t *testing.T) {
	tree := storage.NewMemoryTree()
	permEval := NewPermissionEvaluator(tree)
	filterManager := NewFilterManager(tree, permEval)
	agentID := uint64(12345)

	// Create filter condition
	filter := types.Record{
		Fields: map[string]interface{}{
			"clearance":  types.List{Elements: []interface{}{types.Text{Value: "public"}, types.Text{Value: "internal"}}},
			"department": types.Text{Value: "IT"},
		},
	}

	// Set filter
	err := filterManager.SetFilter(filter, agentID)
	if err != nil {
		t.Fatalf("Failed to set filter: %v", err)
	}

	// Get filter condition
	condition, err := filterManager.getFilterCondition(agentID)
	if err != nil {
		t.Fatalf("Failed to get filter condition: %v", err)
	}

	if condition == nil {
		t.Fatal("Expected filter condition, got nil")
	}

	if len(condition.Expressions) != 2 {
		t.Errorf("Expected 2 filter expressions, got %d", len(condition.Expressions))
	}

	// Check clearance filter expression
	var clearanceExpr *FilterExpression
	var departmentExpr *FilterExpression

	for i := range condition.Expressions {
		if condition.Expressions[i].Attribute == "clearance" {
			clearanceExpr = &condition.Expressions[i]
		} else if condition.Expressions[i].Attribute == "department" {
			departmentExpr = &condition.Expressions[i]
		}
	}

	if clearanceExpr == nil {
		t.Error("Expected clearance filter expression")
	} else {
		if clearanceExpr.Operator != "in" {
			t.Errorf("Expected clearance operator to be 'in', got %s", clearanceExpr.Operator)
		}
	}

	if departmentExpr == nil {
		t.Error("Expected department filter expression")
	} else {
		if departmentExpr.Operator != "?=" {
			t.Errorf("Expected department operator to be '?=', got %s", departmentExpr.Operator)
		}
	}
}

func TestFilterManager_ApplyFilters_ClearanceMatch(t *testing.T) {
	tree := storage.NewMemoryTree()
	permEval := NewPermissionEvaluator(tree)
	filterManager := NewFilterManager(tree, permEval)
	agentID := uint64(12345)

	// Create agent
	agent := &Agent{
		Identity: "kalevo",
		AgentID:  agentID,
		Tree:     tree,
	}

	// Set clearance filter
	filter := CreateClearanceFilter([]string{"public", "internal"})
	filterManager.SetFilter(filter, agentID)

	// Test instance that should pass filter (clearance: internal)
	instancePass := map[string]interface{}{
		"clearance": types.Text{Value: "internal"},
		"data":      types.Text{Value: "test data"},
	}

	visible, err := filterManager.ApplyFilters(agent, instancePass)
	if err != nil {
		t.Errorf("ApplyFilters error: %v", err)
	}
	if !visible {
		t.Error("Expected instance with internal clearance to be visible")
	}

	// Test instance that should not pass filter (clearance: secret)
	instanceFail := map[string]interface{}{
		"clearance": types.Text{Value: "secret"},
		"data":      types.Text{Value: "secret data"},
	}

	visible, err = filterManager.ApplyFilters(agent, instanceFail)
	if err != nil {
		t.Errorf("ApplyFilters error: %v", err)
	}
	if visible {
		t.Error("Expected instance with secret clearance to be filtered out")
	}
}

func TestFilterManager_ApplyFilters_DepartmentMatch(t *testing.T) {
	tree := storage.NewMemoryTree()
	permEval := NewPermissionEvaluator(tree)
	filterManager := NewFilterManager(tree, permEval)
	agentID := uint64(12345)

	// Create agent
	agent := &Agent{
		Identity: "kalevo",
		AgentID:  agentID,
		Tree:     tree,
	}

	// Set department filter
	filter := CreateDepartmentFilter([]string{"IT", "Engineering"})
	filterManager.SetFilter(filter, agentID)

	// Test instance that should pass filter
	instancePass := map[string]interface{}{
		"department": types.Text{Value: "IT"},
		"data":       types.Text{Value: "IT data"},
	}

	visible, err := filterManager.ApplyFilters(agent, instancePass)
	if err != nil {
		t.Errorf("ApplyFilters error: %v", err)
	}
	if !visible {
		t.Error("Expected IT department instance to be visible")
	}

	// Test instance that should not pass filter
	instanceFail := map[string]interface{}{
		"department": types.Text{Value: "Marketing"},
		"data":       types.Text{Value: "marketing data"},
	}

	visible, err = filterManager.ApplyFilters(agent, instanceFail)
	if err != nil {
		t.Errorf("ApplyFilters error: %v", err)
	}
	if visible {
		t.Error("Expected Marketing department instance to be filtered out")
	}
}

func TestFilterManager_ApplyFilters_AuthorMatch(t *testing.T) {
	tree := storage.NewMemoryTree()
	permEval := NewPermissionEvaluator(tree)
	filterManager := NewFilterManager(tree, permEval)
	agentID := uint64(12345)

	// Create agent
	agent := &Agent{
		Identity: "kalevo",
		AgentID:  agentID,
		Tree:     tree,
	}

	// Set author filter
	filter := CreateAuthorFilter([]uint64{12345, 67890})
	filterManager.SetFilter(filter, agentID)

	// Test instance that should pass filter (author matches)
	instancePass := map[string]interface{}{
		"@author": uint64(12345),
		"data":    types.Text{Value: "my data"},
	}

	visible, err := filterManager.ApplyFilters(agent, instancePass)
	if err != nil {
		t.Errorf("ApplyFilters error: %v", err)
	}
	if !visible {
		t.Error("Expected instance with matching author to be visible")
	}

	// Test instance that should not pass filter (author doesn't match)
	instanceFail := map[string]interface{}{
		"@author": uint64(99999),
		"data":    types.Text{Value: "other data"},
	}

	visible, err = filterManager.ApplyFilters(agent, instanceFail)
	if err != nil {
		t.Errorf("ApplyFilters error: %v", err)
	}
	if visible {
		t.Error("Expected instance with non-matching author to be filtered out")
	}
}

func TestFilterManager_ApplyFilters_NoFilter(t *testing.T) {
	tree := storage.NewMemoryTree()
	permEval := NewPermissionEvaluator(tree)
	filterManager := NewFilterManager(tree, permEval)
	agentID := uint64(12345)

	// Create agent with no filter set
	agent := &Agent{
		Identity: "kalevo",
		AgentID:  agentID,
		Tree:     tree,
	}

	// Test instance
	instance := map[string]interface{}{
		"data": types.Text{Value: "any data"},
	}

	visible, err := filterManager.ApplyFilters(agent, instance)
	if err != nil {
		t.Errorf("ApplyFilters error: %v", err)
	}
	if !visible {
		t.Error("Expected instance to be visible when no filter is set")
	}
}

func TestFilterManager_FilterVisible(t *testing.T) {
	tree := storage.NewMemoryTree()
	permEval := NewPermissionEvaluator(tree)
	filterManager := NewFilterManager(tree, permEval)
	agentID := uint64(12345)

	// Create agent
	agent := &Agent{
		Identity: "kalevo",
		AgentID:  agentID,
		Tree:     tree,
	}

	// Set clearance filter
	filter := CreateClearanceFilter([]string{"internal"})
	filterManager.SetFilter(filter, agentID)

	// Create test data
	data := map[string]interface{}{
		"public_item": map[string]interface{}{
			"clearance": types.Text{Value: "public"},
			"value":     types.Text{Value: "public data"},
		},
		"internal_item": map[string]interface{}{
			"clearance": types.Text{Value: "internal"},
			"value":     types.Text{Value: "internal data"},
		},
		"secret_item": map[string]interface{}{
			"clearance": types.Text{Value: "secret"},
			"value":     types.Text{Value: "secret data"},
		},
	}

	// Apply filter
	filtered, err := filterManager.FilterVisible(agent, data)
	if err != nil {
		t.Fatalf("FilterVisible error: %v", err)
	}

	// Should only see internal item
	if len(filtered) != 1 {
		t.Errorf("Expected 1 visible item, got %d", len(filtered))
	}

	if _, exists := filtered["internal_item"]; !exists {
		t.Error("Expected internal_item to be visible")
	}

	if _, exists := filtered["public_item"]; exists {
		t.Error("Expected public_item to be filtered out")
	}

	if _, exists := filtered["secret_item"]; exists {
		t.Error("Expected secret_item to be filtered out")
	}
}

func TestCreateClearanceFilter(t *testing.T) {
	clearanceLevels := []string{"public", "internal"}
	filter := CreateClearanceFilter(clearanceLevels)

	if len(filter.Fields) != 1 {
		t.Errorf("Expected 1 field in filter, got %d", len(filter.Fields))
	}

	clearanceField, exists := filter.Fields["clearance"]
	if !exists {
		t.Fatal("Expected clearance field in filter")
	}

	clearanceList, ok := clearanceField.(types.List)
	if !ok {
		t.Fatal("Expected clearance field to be a list")
	}

	if len(clearanceList.Elements) != 2 {
		t.Errorf("Expected 2 clearance levels, got %d", len(clearanceList.Elements))
	}

	// Check first element
	if elem0, ok := clearanceList.Elements[0].(types.Text); !ok || elem0.Value != "public" {
		t.Errorf("Expected first element to be 'public', got %v", clearanceList.Elements[0])
	}

	// Check second element
	if elem1, ok := clearanceList.Elements[1].(types.Text); !ok || elem1.Value != "internal" {
		t.Errorf("Expected second element to be 'internal', got %v", clearanceList.Elements[1])
	}
}

func TestCreateDepartmentFilter(t *testing.T) {
	departments := []string{"IT", "Engineering"}
	filter := CreateDepartmentFilter(departments)

	if len(filter.Fields) != 1 {
		t.Errorf("Expected 1 field in filter, got %d", len(filter.Fields))
	}

	departmentField, exists := filter.Fields["department"]
	if !exists {
		t.Fatal("Expected department field in filter")
	}

	departmentList, ok := departmentField.(types.List)
	if !ok {
		t.Fatal("Expected department field to be a list")
	}

	if len(departmentList.Elements) != 2 {
		t.Errorf("Expected 2 departments, got %d", len(departmentList.Elements))
	}
}

func TestCreateAuthorFilter(t *testing.T) {
	authorIDs := []uint64{12345, 67890}
	filter := CreateAuthorFilter(authorIDs)

	if len(filter.Fields) != 1 {
		t.Errorf("Expected 1 field in filter, got %d", len(filter.Fields))
	}

	authorField, exists := filter.Fields["@author"]
	if !exists {
		t.Fatal("Expected @author field in filter")
	}

	authorList, ok := authorField.(types.List)
	if !ok {
		t.Fatal("Expected @author field to be a list")
	}

	if len(authorList.Elements) != 2 {
		t.Errorf("Expected 2 author IDs, got %d", len(authorList.Elements))
	}

	// Check first element
	if elem0, ok := authorList.Elements[0].(uint64); !ok || elem0 != 12345 {
		t.Errorf("Expected first element to be 12345, got %v", authorList.Elements[0])
	}
}
