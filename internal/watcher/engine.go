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

// Watcher represents a reactive watcher that monitors path changes
type Watcher struct {
	Name     string              // Watcher identifier
	Watching []string            // Paths being monitored
	Enabled  bool                // Whether watcher is active (defaults to true)
	Code     string              // MBL code to execute when triggered
	Storage  storage.Tree        // Storage access
	AgentID  uint64              // Agent identity for execution
}

// WatcherEngine manages all watchers and tracks changes
type WatcherEngine struct {
	watchers     map[string]*Watcher     // Active watchers by name
	changeLog    map[string]time.Time    // Track when values last changed
	storage      storage.Tree            // Storage access
	agentID      uint64                  // Default agent identity
	lastTickTime time.Time               // When last heartbeat tick occurred
	coordinator  *interpreter.CommitCoordinator // Optional coordinator for batched commits
}

// NewWatcherEngine creates a new watcher execution engine
func NewWatcherEngine(storage storage.Tree, agentID uint64) *WatcherEngine {
	return &WatcherEngine{
		watchers:     make(map[string]*Watcher),
		changeLog:    make(map[string]time.Time),
		storage:      storage,
		agentID:      agentID,
		lastTickTime: time.Now(),
		coordinator:  nil, // No coordinator by default
	}
}

// NewWatcherEngineWithCoordinator creates a watcher engine with commit coordination
func NewWatcherEngineWithCoordinator(storage storage.Tree, agentID uint64, coordinator *interpreter.CommitCoordinator) *WatcherEngine {
	return &WatcherEngine{
		watchers:     make(map[string]*Watcher),
		changeLog:    make(map[string]time.Time),
		storage:      storage,
		agentID:      agentID,
		lastTickTime: time.Now(),
		coordinator:  coordinator,
	}
}

// RegisterWatcher adds a new watcher to the engine
func (we *WatcherEngine) RegisterWatcher(name string, watching []string, code string) error {
	if name == "" {
		return fmt.Errorf("watcher name cannot be empty")
	}

	watcher := &Watcher{
		Name:     name,
		Watching: watching,
		Enabled:  true, // Default to enabled
		Code:     code,
		Storage:  we.storage,
		AgentID:  we.agentID,
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

// GetTriggeredWatchers returns watchers that should fire due to recent changes
func (we *WatcherEngine) GetTriggeredWatchers() []*Watcher {
	var triggered []*Watcher

	for _, watcher := range we.watchers {
		if !watcher.Enabled {
			continue
		}

		// Check if any watched path has changed since last tick
		for _, watchedPath := range watcher.Watching {
			if changeTime, changed := we.changeLog[watchedPath]; changed {
				if changeTime.After(we.lastTickTime) {
					triggered = append(triggered, watcher)
					break // Don't add the same watcher twice
				}
			}
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
