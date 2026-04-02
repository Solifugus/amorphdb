// Package watcher implements MBL watchers and the reactive execution engine
package watcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/solifugus/amorphdb/internal/mbl/parser"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// createTestStorage creates a temporary storage for testing
func createTestStorage(t *testing.T) *storage.StorageTree {
	tempDir := t.TempDir()
	storageDir := filepath.Join(tempDir, "data")
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		t.Fatalf("Failed to create test storage directory: %v", err)
	}

	tree, err := storage.NewStorageTree(storageDir)
	if err != nil {
		t.Fatalf("Failed to create storage tree: %v", err)
	}

	return tree
}

// TestWatcherEngineBasics tests basic watcher engine functionality
func TestWatcherEngineBasics(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Test registering a watcher
	err := engine.RegisterWatcher("test_watcher", []string{"my.data"}, "output(\"triggered\")")
	if err != nil {
		t.Errorf("Failed to register watcher: %v", err)
	}

	// Test that the watcher is registered
	watchers := engine.GetWatchers()
	if len(watchers) != 1 {
		t.Errorf("Expected 1 watcher, got %d", len(watchers))
	}

	watcher, exists := watchers["test_watcher"]
	if !exists {
		t.Error("Watcher 'test_watcher' not found")
	}

	if watcher.Name != "test_watcher" {
		t.Errorf("Expected watcher name 'test_watcher', got '%s'", watcher.Name)
	}

	if !watcher.Enabled {
		t.Error("Watcher should be enabled by default")
	}

	if len(watcher.Watching) != 1 || watcher.Watching[0] != "my.data" {
		t.Errorf("Expected watcher to watch 'my.data', got %v", watcher.Watching)
	}
}

// TestWatcherTriggering tests watcher triggering based on changes
func TestWatcherTriggering(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Register a watcher
	err := engine.RegisterWatcher("change_watcher", []string{"my.value"}, "output(\"value changed\")")
	if err != nil {
		t.Fatalf("Failed to register watcher: %v", err)
	}

	// Initially, no watchers should be triggered
	triggered := engine.GetTriggeredWatchers()
	if len(triggered) != 0 {
		t.Errorf("Expected 0 triggered watchers, got %d", len(triggered))
	}

	// Record a change
	engine.RecordChange("my.value")

	// Now the watcher should be triggered
	triggered = engine.GetTriggeredWatchers()
	if len(triggered) != 1 {
		t.Errorf("Expected 1 triggered watcher, got %d", len(triggered))
	}

	if triggered[0].Name != "change_watcher" {
		t.Errorf("Expected triggered watcher 'change_watcher', got '%s'", triggered[0].Name)
	}

	// After tick, the same change shouldn't trigger again
	engine.Tick()
	triggered = engine.GetTriggeredWatchers()
	if len(triggered) != 0 {
		t.Errorf("Expected 0 triggered watchers after tick, got %d", len(triggered))
	}
}

// TestWatcherExecution tests executing watcher code
func TestWatcherExecution(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Register a watcher with simple code
	err := engine.RegisterWatcher("math_watcher", []string{"my.number"}, "42 + 8")
	if err != nil {
		t.Fatalf("Failed to register watcher: %v", err)
	}

	// Get the watcher
	watchers := engine.GetWatchers()
	watcher := watchers["math_watcher"]

	// Execute the watcher
	result, err := engine.ExecuteWatcher(watcher)
	if err != nil {
		t.Errorf("Watcher execution failed: %v", err)
	}

	// Check the result (should be a Number type)
	if number, ok := result.(types.Number); !ok {
		t.Errorf("Expected Number result, got %T: %v", result, result)
	} else if number.Value != 50 {
		t.Errorf("Expected Number value 50, got %v", number.Value)
	}
}

// TestWatcherEnableDisable tests enabling and disabling watchers
func TestWatcherEnableDisable(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Register a watcher
	err := engine.RegisterWatcher("toggle_watcher", []string{"my.toggle"}, "output(\"toggled\")")
	if err != nil {
		t.Fatalf("Failed to register watcher: %v", err)
	}

	// Disable the watcher
	err = engine.EnableWatcher("toggle_watcher", false)
	if err != nil {
		t.Errorf("Failed to disable watcher: %v", err)
	}

	// Record a change
	engine.RecordChange("my.toggle")

	// Disabled watcher should not be triggered
	triggered := engine.GetTriggeredWatchers()
	if len(triggered) != 0 {
		t.Errorf("Expected 0 triggered watchers (disabled), got %d", len(triggered))
	}

	// Re-enable the watcher
	err = engine.EnableWatcher("toggle_watcher", true)
	if err != nil {
		t.Errorf("Failed to enable watcher: %v", err)
	}

	// Now it should be triggered
	triggered = engine.GetTriggeredWatchers()
	if len(triggered) != 1 {
		t.Errorf("Expected 1 triggered watcher (enabled), got %d", len(triggered))
	}
}

// TestWatcherRemoval tests removing watchers
func TestWatcherRemoval(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Register a watcher
	err := engine.RegisterWatcher("temp_watcher", []string{"my.temp"}, "output(\"temp\")")
	if err != nil {
		t.Fatalf("Failed to register watcher: %v", err)
	}

	// Verify it exists
	watchers := engine.GetWatchers()
	if len(watchers) != 1 {
		t.Errorf("Expected 1 watcher, got %d", len(watchers))
	}

	// Remove the watcher
	engine.RemoveWatcher("temp_watcher")

	// Verify it's gone
	watchers = engine.GetWatchers()
	if len(watchers) != 0 {
		t.Errorf("Expected 0 watchers after removal, got %d", len(watchers))
	}
}

// TestMultipleWatchers tests multiple watchers with different paths
func TestMultipleWatchers(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Register multiple watchers
	err := engine.RegisterWatcher("watcher1", []string{"path1"}, "output(\"path1 changed\")")
	if err != nil {
		t.Fatalf("Failed to register watcher1: %v", err)
	}

	err = engine.RegisterWatcher("watcher2", []string{"path2"}, "output(\"path2 changed\")")
	if err != nil {
		t.Fatalf("Failed to register watcher2: %v", err)
	}

	err = engine.RegisterWatcher("watcher3", []string{"path1", "path2"}, "output(\"either path changed\")")
	if err != nil {
		t.Fatalf("Failed to register watcher3: %v", err)
	}

	// Change path1 - should trigger watcher1 and watcher3
	engine.RecordChange("path1")

	triggered := engine.GetTriggeredWatchers()
	if len(triggered) != 2 {
		t.Errorf("Expected 2 triggered watchers, got %d", len(triggered))
	}

	// Check that the right watchers were triggered
	names := make(map[string]bool)
	for _, w := range triggered {
		names[w.Name] = true
	}

	if !names["watcher1"] || !names["watcher3"] {
		t.Error("Expected watcher1 and watcher3 to be triggered")
	}

	if names["watcher2"] {
		t.Error("watcher2 should not be triggered")
	}
}

// TestWatcherExecutionWithUnknown tests watcher execution with Unknown values
func TestWatcherExecutionWithUnknown(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Register a watcher with code that produces Unknown
	err := engine.RegisterWatcher("unknown_watcher", []string{"my.path"}, "undefined_variable")
	if err != nil {
		t.Fatalf("Failed to register watcher: %v", err)
	}

	// Get the watcher
	watchers := engine.GetWatchers()
	watcher := watchers["unknown_watcher"]

	// Execute the watcher
	result, err := engine.ExecuteWatcher(watcher)

	// Should not return an error, but result should be Unknown
	if err != nil {
		t.Errorf("Watcher execution should not return error for Unknown: %v", err)
	}

	if unknown, ok := result.(types.Unknown); !ok {
		t.Errorf("Expected Unknown result, got %T: %v", result, result)
	} else if unknown.Reason == "" {
		t.Error("Unknown should have a reason")
	}
}

// TestAppendEventRecording tests recording append events
func TestAppendEventRecording(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Create some test item paths (simulating appended items)
	items := []string{"path.item1", "path.item2"}

	// Record an append event
	engine.RecordAppend("my.list", items)

	// Check that we can retrieve the append event
	event, exists := engine.GetAppendEvent("my.list")
	if !exists {
		t.Error("Expected append event to exist")
	}

	if event == nil {
		t.Error("Append event should not be nil")
	} else {
		if event.Path != "my.list" {
			t.Errorf("Expected path 'my.list', got '%s'", event.Path)
		}

		if len(event.Items) != 2 {
			t.Errorf("Expected 2 items, got %d", len(event.Items))
		}

		if event.Items[0] != "path.item1" || event.Items[1] != "path.item2" {
			t.Errorf("Items in append event don't match expected paths: %v", event.Items)
		}

		if event.Timestamp.IsZero() {
			t.Error("Append event timestamp should not be zero")
		}
	}
}

// TestAppendEventTicking tests that append events respect tick timing
func TestAppendEventTicking(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Create a test item path
	items := []string{"path.item"}

	// Record append event
	engine.RecordAppend("my.list", items)

	// Should be available before tick
	_, exists := engine.GetAppendEvent("my.list")
	if !exists {
		t.Error("Append event should exist before tick")
	}

	// After tick, same append shouldn't be available
	engine.Tick()
	_, exists = engine.GetAppendEvent("my.list")
	if exists {
		t.Error("Append event should not exist after tick (timing)")
	}
}

// TestAppendEventClearing tests clearing append events
func TestAppendEventClearing(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Create a test item path
	items := []string{"path.item"}

	// Record append event
	engine.RecordAppend("my.list", items)

	// Should exist initially
	_, exists := engine.GetAppendEvent("my.list")
	if !exists {
		t.Error("Append event should exist initially")
	}

	// Clear the logs
	engine.ClearChangeLog()

	// Should no longer exist
	_, exists = engine.GetAppendEvent("my.list")
	if exists {
		t.Error("Append event should not exist after clearing")
	}
}

// TestMultipleAppendEvents tests handling multiple append events
func TestMultipleAppendEvents(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Record append events on different paths
	engine.RecordAppend("list1", []string{"path.item1"})
	engine.RecordAppend("list2", []string{"path.item2", "path.item3"})

	// Check both events exist
	event1, exists1 := engine.GetAppendEvent("list1")
	if !exists1 {
		t.Error("Append event for list1 should exist")
	}

	event2, exists2 := engine.GetAppendEvent("list2")
	if !exists2 {
		t.Error("Append event for list2 should exist")
	}

	// Check the contents
	if event1 != nil && len(event1.Items) != 1 {
		t.Errorf("Expected 1 item in list1, got %d", len(event1.Items))
	}

	if event2 != nil && len(event2.Items) != 2 {
		t.Errorf("Expected 2 items in list2, got %d", len(event2.Items))
	}
}

// TestAppendEventOverwrite tests that new append events overwrite previous ones
func TestAppendEventOverwrite(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Record first append event
	engine.RecordAppend("my.list", []string{"path.item1"})

	// Record second append event (should overwrite)
	engine.RecordAppend("my.list", []string{"path.item2"})

	// Should only have the second event
	event, exists := engine.GetAppendEvent("my.list")
	if !exists {
		t.Error("Append event should exist")
	}

	if event != nil {
		if len(event.Items) != 1 {
			t.Errorf("Expected 1 item, got %d", len(event.Items))
		}
		if event.Items[0] != "path.item2" {
			t.Error("Should have the second path, not the first")
		}
	}
}

// TestAppendEventWithoutPredicate tests append events with no predicate filtering
func TestAppendEventWithoutPredicate(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Record append event with no filters
	items := []string{"path.item1", "path.item2"}
	engine.RecordAppendWithFilters("my.list", items, nil)

	// Should get all items when no filters applied
	event, exists := engine.GetFilteredAppendEvent("my.list", nil)
	if !exists {
		t.Error("Append event should exist with no filters")
	}

	if event != nil && len(event.Items) != 2 {
		t.Errorf("Expected 2 items with no filters, got %d", len(event.Items))
	}
}

// TestAppendEventWithPredicate tests append events with predicate filtering
func TestAppendEventWithPredicate(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Create simple filter expressions (placeholder - these would be parsed expressions)
	filters := []parser.Expression{}

	// Record append event with filters (using simple item paths)
	items := []string{"item1", "item2"}
	engine.RecordAppendWithFilters("my.list", items, filters)

	// Test that the event was recorded with filters
	event, exists := engine.GetAppendEvent("my.list")
	if !exists {
		t.Error("Append event should exist")
	}

	if event != nil {
		if len(event.Filters) != 0 {
			t.Errorf("Expected 0 filters recorded, got %d", len(event.Filters))
		}
		if len(event.Items) != 2 {
			t.Errorf("Expected 2 items recorded, got %d", len(event.Items))
		}
	}
}

// TestAppendEventWithPredicateNoMatches tests append events where predicate matches no items
func TestAppendEventWithPredicateNoMatches(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Create filter expressions (placeholder)
	filters := []parser.Expression{}

	// Record append event with items that don't exist in storage
	items := []string{"nonexistent.item1", "nonexistent.item2"}
	engine.RecordAppendWithFilters("my.list", items, filters)

	// Test that the basic event was recorded
	event, exists := engine.GetAppendEvent("my.list")
	if !exists {
		t.Error("Basic append event should exist")
	}

	if event != nil && len(event.Items) != 2 {
		t.Errorf("Expected 2 items in basic event, got %d", len(event.Items))
	}

	// For now, just test that filtering doesn't crash
	// In the full implementation, this would test actual predicate filtering
	filteredEvent, exists := engine.GetFilteredAppendEvent("my.list", filters)
	// With our simplified implementation, non-existent items won't match
	if exists && filteredEvent != nil && len(filteredEvent.Items) > 0 {
		t.Logf("Filtered event returned %d items", len(filteredEvent.Items))
	}
}

// TestAppendEventMixedMatches tests append events with some matching and some non-matching items
func TestAppendEventMixedMatches(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Create filter expressions (placeholder)
	filters := []parser.Expression{}

	// Record append event with mix of items
	items := []string{"item1", "item2"}
	engine.RecordAppendWithFilters("my.list", items, filters)

	// Test that basic event recording works
	event, exists := engine.GetAppendEvent("my.list")
	if !exists {
		t.Error("Basic append event should exist")
	}

	if event != nil {
		if len(event.Items) != 2 {
			t.Errorf("Expected 2 items in basic event, got %d", len(event.Items))
		}
		if event.Items[0] != "item1" {
			t.Errorf("Expected 'item1', got '%s'", event.Items[0])
		}
	}

	// Test that filtering structure is in place
	filteredEvent, exists := engine.GetFilteredAppendEvent("my.list", filters)
	if exists && filteredEvent != nil {
		t.Logf("Filtering returned %d items", len(filteredEvent.Items))
	}
}

// TestAppendWatcherRegistration tests registering append watchers
func TestAppendWatcherRegistration(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Register an append watcher
	err := engine.RegisterAppendWatcher("test_append", []string{"my.list"}, "output(items.@size)", "items", []parser.Expression{})
	if err != nil {
		t.Fatalf("Failed to register append watcher: %v", err)
	}

	// Check that the watcher was registered correctly
	watchers := engine.GetWatchers()
	if len(watchers) != 1 {
		t.Errorf("Expected 1 watcher, got %d", len(watchers))
	}

	watcher, exists := watchers["test_append"]
	if !exists {
		t.Error("Append watcher 'test_append' not found")
	}

	if watcher.TriggerType != TriggerTypeAppend {
		t.Errorf("Expected TriggerTypeAppend, got %v", watcher.TriggerType)
	}

	if watcher.BindingName != "items" {
		t.Errorf("Expected binding name 'items', got '%s'", watcher.BindingName)
	}
}

// TestAppendWatcherRequiresBindingName tests that append watchers require a binding name
func TestAppendWatcherRequiresBindingName(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Try to register an append watcher without a binding name
	err := engine.RegisterAppendWatcher("test_append", []string{"my.list"}, "output(\"test\")", "", []parser.Expression{})
	if err == nil {
		t.Error("Expected error when registering append watcher without binding name")
	}

	if !containsString(err.Error(), "binding name") {
		t.Errorf("Expected error about binding name, got: %v", err)
	}
}

// TestAppendWatcherTriggering tests that append watchers trigger on append events
func TestAppendWatcherTriggering(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Register an append watcher
	err := engine.RegisterAppendWatcher("append_watcher", []string{"my.list"}, "output(items)", "items", []parser.Expression{})
	if err != nil {
		t.Fatalf("Failed to register append watcher: %v", err)
	}

	// Initially, no watchers should be triggered
	triggered := engine.GetTriggeredWatchers()
	if len(triggered) != 0 {
		t.Errorf("Expected 0 triggered watchers initially, got %d", len(triggered))
	}

	// Record an append event
	engine.RecordAppend("my.list", []string{"item1", "item2"})

	// Now the append watcher should be triggered
	triggered = engine.GetTriggeredWatchers()
	if len(triggered) != 1 {
		t.Errorf("Expected 1 triggered watcher, got %d", len(triggered))
	}

	if triggered[0].Name != "append_watcher" {
		t.Errorf("Expected triggered watcher 'append_watcher', got '%s'", triggered[0].Name)
	}

	if triggered[0].TriggerType != TriggerTypeAppend {
		t.Errorf("Expected TriggerTypeAppend, got %v", triggered[0].TriggerType)
	}
}

// TestValueChangeWatcherStillWorks tests that value change watchers still work after append watcher changes
func TestValueChangeWatcherStillWorks(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Register a value change watcher
	err := engine.RegisterWatcher("change_watcher", []string{"my.value"}, "output(\"changed\")")
	if err != nil {
		t.Fatalf("Failed to register value change watcher: %v", err)
	}

	// Check the watcher properties
	watchers := engine.GetWatchers()
	watcher := watchers["change_watcher"]

	if watcher.TriggerType != TriggerTypeValueChange {
		t.Errorf("Expected TriggerTypeValueChange, got %v", watcher.TriggerType)
	}

	// Record a value change
	engine.RecordChange("my.value")

	// The watcher should be triggered
	triggered := engine.GetTriggeredWatchers()
	if len(triggered) != 1 {
		t.Errorf("Expected 1 triggered watcher, got %d", len(triggered))
	}

	if triggered[0].Name != "change_watcher" {
		t.Errorf("Expected triggered watcher 'change_watcher', got '%s'", triggered[0].Name)
	}
}

// TestAppendWatcherExecution tests executing an append watcher
func TestAppendWatcherExecution(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Register an append watcher with simple code
	err := engine.RegisterAppendWatcher("exec_append", []string{"my.list"}, "42", "items", []parser.Expression{})
	if err != nil {
		t.Fatalf("Failed to register append watcher: %v", err)
	}

	// Record an append event
	engine.RecordAppend("my.list", []string{"item1"})

	// Get the watcher
	watchers := engine.GetWatchers()
	watcher := watchers["exec_append"]

	// Execute the watcher - this tests the binding setup
	result, err := engine.ExecuteWatcher(watcher)
	if err != nil {
		t.Errorf("Append watcher execution failed: %v", err)
	}

	// Check the result (should be a Number type)
	if number, ok := result.(types.Number); !ok {
		t.Errorf("Expected Number result, got %T: %v", result, result)
	} else if number.Value != 42 {
		t.Errorf("Expected Number value 42, got %v", number.Value)
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestMultiPathWatcherParsing tests end-to-end multi-path watcher functionality
func TestMultiPathWatcherParsing(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	tests := []struct {
		name         string
		paths        []string
		changePath   string
		shouldTrigger bool
		description  string
	}{
		{
			name:         "multi_balance_limit",
			paths:        []string{"my.account.balance", "my.account.limit"},
			changePath:   "my.account.balance",
			shouldTrigger: true,
			description:  "Balance change should trigger balance/limit watcher",
		},
		{
			name:         "multi_balance_limit_2",
			paths:        []string{"my.account.balance", "my.account.limit"},
			changePath:   "my.account.limit",
			shouldTrigger: true,
			description:  "Limit change should trigger balance/limit watcher",
		},
		{
			name:         "multi_balance_limit_3",
			paths:        []string{"my.account.balance", "my.account.limit"},
			changePath:   "my.account.status",
			shouldTrigger: false,
			description:  "Status change should NOT trigger balance/limit watcher",
		},
		{
			name:         "three_path_monitor",
			paths:        []string{"my.cpu.usage", "my.memory.usage", "my.disk.usage"},
			changePath:   "my.cpu.usage",
			shouldTrigger: true,
			description:  "CPU change should trigger resource monitor",
		},
		{
			name:         "three_path_monitor_2",
			paths:        []string{"my.cpu.usage", "my.memory.usage", "my.disk.usage"},
			changePath:   "my.memory.usage",
			shouldTrigger: true,
			description:  "Memory change should trigger resource monitor",
		},
		{
			name:         "three_path_monitor_3",
			paths:        []string{"my.cpu.usage", "my.memory.usage", "my.disk.usage"},
			changePath:   "my.disk.usage",
			shouldTrigger: true,
			description:  "Disk change should trigger resource monitor",
		},
		{
			name:         "three_path_monitor_4",
			paths:        []string{"my.cpu.usage", "my.memory.usage", "my.disk.usage"},
			changePath:   "my.network.usage",
			shouldTrigger: false,
			description:  "Network change should NOT trigger CPU/memory/disk monitor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			// Register the multi-path watcher
			watcherCode := "my.alerts.count = my.alerts.count + 1"
			err := engine.RegisterWatcher(tt.name, tt.paths, watcherCode)
			if err != nil {
				t.Fatalf("Failed to register watcher: %v", err)
			}

			// Record change in the specified path
			engine.RecordChange(tt.changePath)

			// Check if watcher was triggered
			triggered := engine.GetTriggeredWatchers()

			if tt.shouldTrigger {
				found := false
				for _, w := range triggered {
					if w.Name == tt.name {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected watcher '%s' to be triggered by change to '%s'", tt.name, tt.changePath)
				}
			} else {
				for _, w := range triggered {
					if w.Name == tt.name {
						t.Errorf("Watcher '%s' should NOT be triggered by change to '%s'", tt.name, tt.changePath)
					}
				}
			}

			// Clean up for next test iteration
			engine.RemoveWatcher(tt.name)
			engine.ClearChangeLog()
		})
	}
}

// TestMultiPathWithSimultaneousChanges tests multi-path watchers with simultaneous changes
func TestMultiPathWithSimultaneousChanges(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Register a multi-path watcher
	watcherName := "account_monitor"
	paths := []string{"my.account.balance", "my.account.limit", "my.account.status"}
	watcherCode := "output(\"Account change detected\")"

	err := engine.RegisterWatcher(watcherName, paths, watcherCode)
	if err != nil {
		t.Fatalf("Failed to register watcher: %v", err)
	}

	// Record simultaneous changes to multiple watched paths
	engine.RecordChange("my.account.balance")
	engine.RecordChange("my.account.limit")
	engine.RecordChange("my.account.status")

	// Should trigger exactly once despite multiple path changes
	triggered := engine.GetTriggeredWatchers()

	watcherCount := 0
	for _, w := range triggered {
		if w.Name == watcherName {
			watcherCount++
		}
	}

	if watcherCount != 1 {
		t.Errorf("Expected watcher to be triggered exactly once, got %d times", watcherCount)
	}
}

// TestMixedSingleAndMultiPathWatchers tests interaction between single and multi-path watchers
func TestMixedSingleAndMultiPathWatchers(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Register single-path watchers
	err := engine.RegisterWatcher("balance_only", []string{"my.account.balance"}, "output(\"balance\")")
	if err != nil {
		t.Fatalf("Failed to register balance_only watcher: %v", err)
	}

	err = engine.RegisterWatcher("limit_only", []string{"my.account.limit"}, "output(\"limit\")")
	if err != nil {
		t.Fatalf("Failed to register limit_only watcher: %v", err)
	}

	// Register multi-path watcher watching the same paths
	err = engine.RegisterWatcher("balance_and_limit", []string{"my.account.balance", "my.account.limit"}, "output(\"either\")")
	if err != nil {
		t.Fatalf("Failed to register balance_and_limit watcher: %v", err)
	}

	// Change balance - should trigger balance_only and balance_and_limit
	engine.RecordChange("my.account.balance")

	triggered := engine.GetTriggeredWatchers()
	if len(triggered) != 2 {
		t.Errorf("Expected 2 triggered watchers, got %d", len(triggered))
	}

	// Verify the correct watchers were triggered
	names := make(map[string]bool)
	for _, w := range triggered {
		names[w.Name] = true
	}

	if !names["balance_only"] {
		t.Error("balance_only watcher should be triggered")
	}
	if !names["balance_and_limit"] {
		t.Error("balance_and_limit watcher should be triggered")
	}
	if names["limit_only"] {
		t.Error("limit_only watcher should NOT be triggered")
	}
}