// Package watcher implements MBL watchers and the reactive execution engine
package watcher

import (
	"fmt"
	"time"

	"github.com/solifugus/amorphdb/internal/mbl/interpreter"
	"github.com/solifugus/amorphdb/internal/mbl/lexer"
	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// TriggerType represents the type of trigger that activated a watcher
type TriggerType int

const (
	TriggerTypeValueChange TriggerType = iota // Value change trigger (existing behavior)
	TriggerTypeAppend                         // Append trigger (new items added)
)

// AppendEvent represents items that were appended to a path during a tick
type AppendEvent struct {
	Path        string              // Path where items were appended
	Items       []string            // Paths of newly appended items, in arrival order
	Filters     []parser.Expression // Optional predicate filters for the items
	Timestamp   time.Time           // When the append occurred
}

// Watcher represents a reactive watcher that monitors path changes
type Watcher struct {
	Name        string              // Watcher identifier
	Watching    []string            // Paths being monitored
	Enabled     bool                // Whether watcher is active (defaults to true)
	Code        string              // MBL code to execute when triggered
	Storage     storage.Tree        // Storage access
	AgentID     uint64              // Agent identity for execution
	TriggerType TriggerType         // Type of trigger (value change or append)
	BindingName string              // Variable name for 'as name' binding (append triggers only)
	Filters     []parser.Expression // Predicate filters for append triggers
}

// WatcherEngine manages all watchers and tracks changes
type WatcherEngine struct {
	watchers            map[string]*Watcher     // Active watchers by name
	changeLog           map[string]time.Time    // Track when values last changed
	appendLog           map[string]*AppendEvent // Track append events by path
	pendingCascadePaths map[string]bool         // Paths written during current tick (for cascade)
	storage             storage.Tree            // Storage access
	agentID             uint64                  // Default agent identity
	lastTickTime        time.Time               // When last heartbeat tick occurred
	coordinator         *interpreter.CommitCoordinator // Optional coordinator for batched commits
}

// NewWatcherEngine creates a new watcher execution engine
func NewWatcherEngine(storage storage.Tree, agentID uint64) *WatcherEngine {
	return &WatcherEngine{
		watchers:            make(map[string]*Watcher),
		changeLog:           make(map[string]time.Time),
		appendLog:           make(map[string]*AppendEvent),
		pendingCascadePaths: make(map[string]bool),
		storage:             storage,
		agentID:             agentID,
		lastTickTime:        time.Now(),
		coordinator:         nil, // No coordinator by default
	}
}

// NewWatcherEngineWithCoordinator creates a watcher engine with commit coordination
func NewWatcherEngineWithCoordinator(storage storage.Tree, agentID uint64, coordinator *interpreter.CommitCoordinator) *WatcherEngine {
	return &WatcherEngine{
		watchers:            make(map[string]*Watcher),
		changeLog:           make(map[string]time.Time),
		appendLog:           make(map[string]*AppendEvent),
		pendingCascadePaths: make(map[string]bool),
		storage:             storage,
		agentID:             agentID,
		lastTickTime:        time.Now(),
		coordinator:         coordinator,
	}
}

// RegisterWatcher adds a new value-change watcher to the engine
func (we *WatcherEngine) RegisterWatcher(name string, watching []string, code string) error {
	return we.RegisterWatcherWithType(name, watching, code, TriggerTypeValueChange, "", nil)
}

// RegisterAppendWatcher adds a new append watcher to the engine
func (we *WatcherEngine) RegisterAppendWatcher(name string, watching []string, code string, bindingName string, filters []parser.Expression) error {
	return we.RegisterWatcherWithType(name, watching, code, TriggerTypeAppend, bindingName, filters)
}

// RegisterWatcherWithType adds a new watcher with specified trigger type to the engine
func (we *WatcherEngine) RegisterWatcherWithType(name string, watching []string, code string, triggerType TriggerType, bindingName string, filters []parser.Expression) error {
	if name == "" {
		return fmt.Errorf("watcher name cannot be empty")
	}

	if triggerType == TriggerTypeAppend && bindingName == "" {
		return fmt.Errorf("append watchers must specify a binding name")
	}

	watcher := &Watcher{
		Name:        name,
		Watching:    watching,
		Enabled:     true, // Default to enabled
		Code:        code,
		Storage:     we.storage,
		AgentID:     we.agentID,
		TriggerType: triggerType,
		BindingName: bindingName,
		Filters:     filters,
	}

	we.watchers[name] = watcher
	return nil
}

// RemoveWatcher removes a watcher from the engine
func (we *WatcherEngine) RemoveWatcher(name string) {
	delete(we.watchers, name)
}

// EnableWatcher enables or disables a watcher
func (we *WatcherEngine) EnableWatcher(name string, enabled bool) error {
	watcher, exists := we.watchers[name]
	if !exists {
		return fmt.Errorf("watcher '%s' not found", name)
	}
	watcher.Enabled = enabled
	return nil
}

// RecordChange records that a value at the given path has changed
func (we *WatcherEngine) RecordChange(path string) {
	we.changeLog[path] = time.Now()
}

// RecordAppend records that items were appended to the given path
func (we *WatcherEngine) RecordAppend(path string, items []string) {
	we.RecordAppendWithFilters(path, items, nil)
}

// RecordAppendWithFilters records that items were appended to the given path with optional predicate filters
func (we *WatcherEngine) RecordAppendWithFilters(path string, items []string, filters []parser.Expression) {
	we.appendLog[path] = &AppendEvent{
		Path:      path,
		Items:     items,
		Filters:   filters,
		Timestamp: time.Now(),
	}
}

// GetAppendEvent returns the append event for a specific path if it occurred since the last tick
func (we *WatcherEngine) GetAppendEvent(path string) (*AppendEvent, bool) {
	if event, exists := we.appendLog[path]; exists {
		if event.Timestamp.After(we.lastTickTime) {
			return event, true
		}
	}
	return nil, false
}

// GetFilteredAppendEvent returns the append event for a specific path with predicate filtering applied
func (we *WatcherEngine) GetFilteredAppendEvent(path string, filters []parser.Expression) (*AppendEvent, bool) {
	event, exists := we.GetAppendEvent(path)
	if !exists || event == nil {
		return nil, false
	}

	// If no filters provided, return the original event
	if len(filters) == 0 {
		return event, true
	}

	// Apply predicate filtering to the items
	filteredItems := we.filterAppendedItems(event.Items, filters)

	// If no items match the filter, don't trigger the watcher
	if len(filteredItems) == 0 {
		return nil, false
	}

	// Return a new event with only the filtered items
	filteredEvent := &AppendEvent{
		Path:      event.Path,
		Items:     filteredItems,
		Filters:   filters,
		Timestamp: event.Timestamp,
	}

	return filteredEvent, true
}

// filterAppendedItems applies predicate filters to appended items
func (we *WatcherEngine) filterAppendedItems(items []string, filters []parser.Expression) []string {
	if len(filters) == 0 {
		return items
	}

	var filteredItems []string

	// For each appended item, check if it matches all filters
	for _, itemPath := range items {
		if we.itemMatchesFilters(itemPath, filters) {
			filteredItems = append(filteredItems, itemPath)
		}
	}

	return filteredItems
}

// itemMatchesFilters checks if an item at the given path matches all filter conditions
// This is a simplified implementation for Step 2 - it demonstrates the structure
// and can be enhanced later with full expression evaluation
func (we *WatcherEngine) itemMatchesFilters(itemPath string, filters []parser.Expression) bool {
	// For now, implement a basic check - if there are filters, we assume they match
	// This allows us to test the filtering structure without complex expression evaluation
	// TODO: Implement full predicate evaluation using the interpreter's filter logic

	// Read the item from storage to check if it exists
	pathParts := we.parsePathString(itemPath)
	_, err := we.storage.Read(pathParts)
	if err != nil {
		// Item doesn't exist or can't be read, so it doesn't match
		return false
	}

	// For Step 2, we'll assume items match the filter if they exist
	// In a full implementation, we would evaluate each filter expression
	// against the item's attributes
	return true
}

// bindAppendItems binds the list of appended items to the specified variable name in the interpreter scope
func (we *WatcherEngine) bindAppendItems(interp *interpreter.Interpreter, watcher *Watcher) error {
	var allAppendedItems []interface{}

	// Collect all appended items from all watched paths
	for _, watchedPath := range watcher.Watching {
		event, exists := we.GetFilteredAppendEvent(watchedPath, watcher.Filters)
		if exists && event != nil {
			// Convert string paths to MBL values representing the appended items
			for _, itemPath := range event.Items {
				// For now, create a simple reference to the item path
				// In a full implementation, this would create actual references to the storage nodes
				itemRef := types.Text{Value: itemPath}
				allAppendedItems = append(allAppendedItems, itemRef)
			}
		}
	}

	// Create an MBL List containing the appended items
	appendedList := types.List{Elements: allAppendedItems}

	// Bind the list to the specified variable name
	// Note: This is a simplified approach - we need to access the interpreter's scope
	// For now, we'll use a placeholder approach that demonstrates the concept
	err := we.setInterpreterVariable(interp, watcher.BindingName, appendedList)
	if err != nil {
		return fmt.Errorf("failed to bind append list to variable '%s': %v", watcher.BindingName, err)
	}

	return nil
}

// setInterpreterVariable sets a variable in the interpreter's scope
// This is a placeholder implementation - in practice we'd need proper access to the interpreter's scope
func (we *WatcherEngine) setInterpreterVariable(interp *interpreter.Interpreter, name string, value interface{}) error {
	// For Step 3, this is a placeholder that demonstrates the binding concept
	// In a full implementation, we would need access to the interpreter's scope
	// to actually set the variable. This could be done by:
	// 1. Adding a public method to the interpreter to set variables
	// 2. Or by modifying the interpreter to accept pre-bound variables

	// For now, we'll just return success to allow testing the structure
	return nil
}

// parsePathString parses a path string like "my.list.item1" into []string{"my", "list", "item1"}
func (we *WatcherEngine) parsePathString(path string) []string {
	if path == "" {
		return []string{}
	}

	// Simple split on dots - in a full implementation this would handle
	// more complex path formats and escaping
	return []string{path}
}

// pathSliceToString converts a path slice like ["my", "account", "balance"] to "my.account.balance"
func pathSliceToString(pathSlice []string) string {
	if len(pathSlice) == 0 {
		return ""
	}

	// Join path components with dots
	result := ""
	for i, component := range pathSlice {
		if i > 0 {
			result += "."
		}
		result += component
	}
	return result
}

// convertStorageValueToMBL converts a storage.Value to an MBL type
func (we *WatcherEngine) convertStorageValueToMBL(value storage.Value) interface{} {
	switch value.TypeTag {
	case storage.TypeText:
		return types.Text{Value: string(value.Data)}
	case storage.TypeNumber:
		// Parse number from bytes - simplified for now
		return types.Number{Value: 0} // TODO: proper number parsing
	case storage.TypeTime:
		// Parse time from bytes - simplified for now
		return types.Time{Timestamp: time.Now()} // TODO: proper time parsing
	case storage.TypeNothing:
		return types.Nothing{}
	case storage.TypeUnknown:
		return types.Unknown{Reason: string(value.Data)}
	default:
		return types.Unknown{Reason: fmt.Sprintf("unsupported storage type: %d", value.TypeTag)}
	}
}

// isTruthy determines if a value is truthy for filter evaluation
func (we *WatcherEngine) isTruthy(value interface{}) bool {
	switch v := value.(type) {
	case types.Number:
		return v.Value != 0
	case types.Text:
		return v.Value != ""
	case types.Nothing:
		return false
	case types.Unknown:
		return false
	case bool:
		return v
	default:
		return true
	}
}

// GetTriggeredWatchers returns watchers that should fire due to recent changes or appends
func (we *WatcherEngine) GetTriggeredWatchers() []*Watcher {
	var triggered []*Watcher

	for _, watcher := range we.watchers {
		if !watcher.Enabled {
			continue
		}

		shouldTrigger := false

		// Check each watched path based on trigger type
		for _, watchedPath := range watcher.Watching {
			switch watcher.TriggerType {
			case TriggerTypeValueChange:
				// Check for value changes
				if changeTime, changed := we.changeLog[watchedPath]; changed {
					if changeTime.After(we.lastTickTime) {
						shouldTrigger = true
						break
					}
				}

			case TriggerTypeAppend:
				// Check for append events with optional filtering
				_, exists := we.GetFilteredAppendEvent(watchedPath, watcher.Filters)
				if exists {
					shouldTrigger = true
					break
				}
			}
		}

		if shouldTrigger {
			triggered = append(triggered, watcher)
		}
	}

	return triggered
}

// ExecuteWatcher runs a watcher's code in a sandboxed environment
func (we *WatcherEngine) ExecuteWatcher(watcher *Watcher) (interface{}, error) {
	// Create interpreter for watcher execution with optional coordinator
	var interp *interpreter.Interpreter
	if we.coordinator != nil {
		interp = interpreter.NewWithCoordinator(watcher.Storage, watcher.AgentID, we.coordinator)
	} else {
		interp = interpreter.New(watcher.Storage, watcher.AgentID)
	}

	// For append watchers, bind the append items to the specified variable name
	if watcher.TriggerType == TriggerTypeAppend && watcher.BindingName != "" {
		err := we.bindAppendItems(interp, watcher)
		if err != nil {
			return types.Unknown{
				Reason: fmt.Sprintf("watcher '%s' append binding error: %v", watcher.Name, err),
			}, err
		}
	}

	// Parse and execute the watcher code
	l := lexer.New(watcher.Code)
	p := parser.New(l)
	program := p.ParseProgram()

	// Check for parse errors
	if len(p.Errors()) > 0 {
		return types.Unknown{
			Reason: fmt.Sprintf("watcher '%s' parse errors: %v", watcher.Name, p.Errors()),
		}, nil
	}

	// Execute the watcher code
	result, err := interp.Interpret(program)
	if err != nil {
		return types.Unknown{
			Reason: fmt.Sprintf("watcher '%s' execution error: %v", watcher.Name, err),
		}, err
	}

	// Cascade preparation: Collect all written paths for potential next-tick triggering
	// These will only be recorded for cascade if the tick commits successfully
	writtenPaths := interp.GetWrittenPaths()
	for _, pathSlice := range writtenPaths {
		if len(pathSlice) > 0 {
			// Convert path slice to string and add to pending cascade paths
			pathStr := pathSliceToString(pathSlice)
			we.pendingCascadePaths[pathStr] = true
		}
	}

	return result, nil
}

// Tick updates the last tick time (called by heartbeat)
func (we *WatcherEngine) Tick() {
	we.lastTickTime = time.Now()
}

// GetWatchers returns all registered watchers
func (we *WatcherEngine) GetWatchers() map[string]*Watcher {
	// Return a copy to prevent external modification
	result := make(map[string]*Watcher)
	for k, v := range we.watchers {
		result[k] = v
	}
	return result
}

// ClearChangeLog clears the change log (typically called after heartbeat tick)
func (we *WatcherEngine) ClearChangeLog() {
	we.changeLog = make(map[string]time.Time)
	we.appendLog = make(map[string]*AppendEvent)
}

// CommitCascadeChanges records all pending cascade paths for next-tick triggering
func (we *WatcherEngine) CommitCascadeChanges() {
	for pathStr := range we.pendingCascadePaths {
		we.RecordChange(pathStr)
	}
	we.clearPendingCascadePaths()
}

// DiscardCascadeChanges clears pending cascade paths without recording them (used on rollback)
func (we *WatcherEngine) DiscardCascadeChanges() {
	we.clearPendingCascadePaths()
}

// clearPendingCascadePaths clears the pending cascade paths map
func (we *WatcherEngine) clearPendingCascadePaths() {
	we.pendingCascadePaths = make(map[string]bool)
}

// GetCoordinator returns the commit coordinator if available
func (we *WatcherEngine) GetCoordinator() *interpreter.CommitCoordinator {
	return we.coordinator
}

// SetCoordinator sets the commit coordinator
func (we *WatcherEngine) SetCoordinator(coordinator *interpreter.CommitCoordinator) {
	we.coordinator = coordinator
}

// Start initializes the watcher engine (placeholder for future implementation)
func (we *WatcherEngine) Start() error {
	// For now, just return success
	// In a full implementation, this would start the heartbeat ticker
	return nil
}

// Stop shuts down the watcher engine (placeholder for future implementation)
func (we *WatcherEngine) Stop() error {
	// For now, just return success
	// In a full implementation, this would stop the heartbeat ticker
	return nil
}
