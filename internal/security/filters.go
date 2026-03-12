// Package security implements filters - personal visibility control system
package security

import (
	"fmt"
	"strings"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// FilterManager handles filter storage and application
type FilterManager struct {
	tree            storage.ExtendedTree
	permissionEval  *PermissionEvaluator
	filterCache     map[uint64]*FilterCondition // Cache of parsed filters by agent
}

// FilterCondition represents a parsed filter condition
type FilterCondition struct {
	Expressions []FilterExpression
}

// FilterExpression represents a single filter expression
type FilterExpression struct {
	Attribute string      // The attribute to check (from stamp)
	Operator  string      // The operator (?=, in, etc.)
	Value     interface{} // The value to compare against
}

// NewFilterManager creates a new filter manager
func NewFilterManager(tree storage.ExtendedTree, permissionEval *PermissionEvaluator) *FilterManager {
	return &FilterManager{
		tree:           tree,
		permissionEval: permissionEval,
		filterCache:    make(map[uint64]*FilterCondition),
	}
}

// ApplyFilters applies an agent's filters to determine visibility of an instance
func (fm *FilterManager) ApplyFilters(agent *Agent, instanceWithStamp map[string]interface{}) (bool, error) {
	// First, check if agent has permission to read
	// Filters cannot bypass permissions - they operate within permission bounds

	// Get agent's filter condition
	filterCondition, err := fm.getFilterCondition(agent.AgentID)
	if err != nil {
		return false, fmt.Errorf("failed to get filter condition: %v", err)
	}

	// If no filter, everything is visible (within permission bounds)
	if filterCondition == nil || len(filterCondition.Expressions) == 0 {
		return true, nil
	}

	// Apply each filter expression
	for _, expr := range filterCondition.Expressions {
		match, err := fm.evaluateFilterExpression(expr, instanceWithStamp)
		if err != nil {
			return false, fmt.Errorf("failed to evaluate filter expression: %v", err)
		}
		if !match {
			return false, nil // Instance doesn't pass filter
		}
	}

	return true, nil // All filter expressions passed
}

// getFilterCondition retrieves and parses an agent's filter from ~.filter
func (fm *FilterManager) getFilterCondition(agentID uint64) (*FilterCondition, error) {
	// Check cache first
	if condition, exists := fm.filterCache[agentID]; exists {
		return condition, nil
	}

	// Get ~.filter for this agent
	filterPath := []string{"~", "filter"}
	attrHash := storage.HashPath(filterPath)

	instance, err := fm.tree.GetInstance(attrHash, agentID)
	if err != nil {
		return nil, nil // No filter set
	}

	value, err := fm.tree.GetValue(instance.ValueHash)
	if err != nil {
		return nil, err
	}

	// Deserialize the filter
	valueStruct := types.Value{TypeTag: storage.TypeText, Data: value}; filterValue, _ := types.DeserializeValue(valueStruct)

	// Parse the filter condition
	condition, err := fm.parseFilterCondition(filterValue)
	if err != nil {
		return nil, fmt.Errorf("failed to parse filter condition: %v", err)
	}

	// Cache the parsed condition
	fm.filterCache[agentID] = condition

	return condition, nil
}

// parseFilterCondition parses a filter value into filter expressions
func (fm *FilterManager) parseFilterCondition(filterValue interface{}) (*FilterCondition, error) {
	// For now, implement a simple parser for basic conditions
	// In a full implementation, this would parse complex MBL expressions

	condition := &FilterCondition{}

	switch fv := filterValue.(type) {
	case types.Record:
		// Parse record as filter expressions
		// Example: { clearance: ["public", "internal"], department: "IT" }
		for key, value := range fv.Fields {
			expr, err := fm.parseFilterExpression(key, value)
			if err != nil {
				return nil, err
			}
			condition.Expressions = append(condition.Expressions, expr)
		}

	case types.Text:
		// Parse text as single expression
		// TODO: Implement parsing of text-based filter conditions
		// For now, treat as simple equality check
		return nil, fmt.Errorf("text-based filters not yet implemented")

	default:
		return nil, fmt.Errorf("unsupported filter type: %T", filterValue)
	}

	return condition, nil
}

// parseFilterExpression parses a single filter expression
func (fm *FilterManager) parseFilterExpression(attribute string, value interface{}) (FilterExpression, error) {
	expr := FilterExpression{
		Attribute: attribute,
	}

	switch v := value.(type) {
	case types.List:
		// List means "in" operator
		expr.Operator = "in"
		expr.Value = v.Elements

	case types.Text:
		// Check if it contains operators
		if strings.Contains(v.Value, "?=") {
			// Parse equality expression: "department ?= 'IT'"
			parts := strings.Split(v.Value, "?=")
			if len(parts) == 2 {
				expr.Attribute = strings.TrimSpace(parts[0])
				expr.Operator = "?="
				expr.Value = types.Text{Value: strings.Trim(strings.TrimSpace(parts[1]), "\"'")}
			}
		} else {
			// Simple equality
			expr.Operator = "?="
			expr.Value = v
		}

	default:
		// Default to equality
		expr.Operator = "?="
		expr.Value = v
	}

	return expr, nil
}

// evaluateFilterExpression evaluates a filter expression against an instance's stamp
func (fm *FilterManager) evaluateFilterExpression(expr FilterExpression, instanceWithStamp map[string]interface{}) (bool, error) {
	// Get the attribute value from the instance's stamp
	stampValue, exists := instanceWithStamp[expr.Attribute]
	if !exists {
		return false, nil // Attribute not in stamp, doesn't match
	}

	switch expr.Operator {
	case "?=":
		// Equality check
		return fm.valuesEqual(stampValue, expr.Value), nil

	case "in":
		// Check if stamp value is in the list
		if valueList, ok := expr.Value.([]interface{}); ok {
			for _, listValue := range valueList {
				if fm.valuesEqual(stampValue, listValue) {
					return true, nil
				}
			}
		}
		return false, nil

	case "or":
		// TODO: Implement OR logic for compound expressions
		return false, fmt.Errorf("OR operator not yet implemented")

	case "and":
		// TODO: Implement AND logic for compound expressions
		return false, fmt.Errorf("AND operator not yet implemented")

	default:
		return false, fmt.Errorf("unsupported filter operator: %s", expr.Operator)
	}
}

// valuesEqual compares two MBL values for equality
func (fm *FilterManager) valuesEqual(a, b interface{}) bool {
	switch va := a.(type) {
	case types.Text:
		if vb, ok := b.(types.Text); ok {
			return va.Value == vb.Value
		}
	case types.Number:
		if vb, ok := b.(types.Number); ok {
			return va.Value == vb.Value
		}
	case types.Boolean:
		if vb, ok := b.(types.Boolean); ok {
			return va.Value == vb.Value
		}
	case uint64:
		if vb, ok := b.(uint64); ok {
			return va == vb
		}
	}
	return false
}

// SetFilter sets an agent's personal filter at ~.filter
func (fm *FilterManager) SetFilter(filterCondition interface{}, agentID uint64) error {
	filterPath := []string{"~", "filter"}
	attrHash := storage.HashPath(filterPath)

	// Serialize the filter
	value := types.SerializeValue(filterCondition)
	valueHash := storage.HashValue(value)

	// Create instance
	instance := storage.SecurityInstance{
		AttributeHash: attrHash,
		ValueHash:     valueHash,
		Agent:         agentID,
		Timestamp:     storage.GetCurrentTimestamp(),
	}

	// Store value and instance
	err := fm.tree.PutValue(valueHash, value)
	if err != nil {
		return fmt.Errorf("failed to store filter value: %v", err)
	}

	err = fm.tree.PutInstance(attrHash, instance)
	if err != nil {
		return fmt.Errorf("failed to store filter instance: %v", err)
	}

	// Invalidate cache
	delete(fm.filterCache, agentID)

	return nil
}

// FilterVisible returns a filtered view of data based on agent's filters and permissions
func (fm *FilterManager) FilterVisible(agent *Agent, data map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, value := range data {
		// Check if this item passes the filter
		// For now, assume each value has embedded stamp information
		var instanceWithStamp map[string]interface{}

		if valueMap, ok := value.(map[string]interface{}); ok {
			instanceWithStamp = valueMap
		} else {
			// If no stamp information, create minimal instance
			instanceWithStamp = map[string]interface{}{
				"value": value,
			}
		}

		visible, err := fm.ApplyFilters(agent, instanceWithStamp)
		if err != nil {
			return nil, err
		}

		if visible {
			result[key] = value
		}
	}

	return result, nil
}

// ClearFilter removes an agent's filter
func (fm *FilterManager) ClearFilter(agentID uint64) error {
	// Remove from cache
	delete(fm.filterCache, agentID)

	// In a full implementation, would also remove from storage
	// For now, just clear cache
	return nil
}

// Example helper for creating common filter types

// CreateClearanceFilter creates a filter for specific clearance levels
func CreateClearanceFilter(clearanceLevels []string) types.Record {
	elements := make([]interface{}, len(clearanceLevels))
	for i, level := range clearanceLevels {
		elements[i] = types.Text{Value: level}
	}

	return types.Record{
		Fields: map[string]interface{}{
			"clearance": types.List{Elements: elements},
		},
	}
}

// CreateDepartmentFilter creates a filter for specific departments
func CreateDepartmentFilter(departments []string) types.Record {
	elements := make([]interface{}, len(departments))
	for i, dept := range departments {
		elements[i] = types.Text{Value: dept}
	}

	return types.Record{
		Fields: map[string]interface{}{
			"department": types.List{Elements: elements},
		},
	}
}

// CreateAuthorFilter creates a filter for specific authors
func CreateAuthorFilter(authorIDs []uint64) types.Record {
	elements := make([]interface{}, len(authorIDs))
	for i, id := range authorIDs {
		elements[i] = id
	}

	return types.Record{
		Fields: map[string]interface{}{
			"@author": types.List{Elements: elements},
		},
	}
}
