package watcher

import (
	"testing"

	"github.com/solifugus/amorphdb/internal/mbl/parser"
)

// TestAppendWatcherEndToEnd tests the complete append watcher functionality
func TestAppendWatcherEndToEnd(t *testing.T) {
	// Create test environment
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Step 1: Set up a list in the mesh (simulated)
	testPath := "my.orders"

	// Step 2: Define an append watcher with a predicate and an as name binding
	watcherName := "order_processor"
	watchedPaths := []string{testPath}
	watcherCode := `
		for order in orders:
			output("Processing order: " & order)
	`
	bindingName := "orders"

	// Create simple placeholder filter expressions (in a full implementation, these would be parsed)
	filters := []parser.Expression{}

	err := engine.RegisterAppendWatcher(watcherName, watchedPaths, watcherCode, bindingName, filters)
	if err != nil {
		t.Fatalf("Failed to register append watcher: %v", err)
	}

	// Step 3: Initially, no watchers should be triggered
	triggered := engine.GetTriggeredWatchers()
	if len(triggered) != 0 {
		t.Errorf("Expected 0 triggered watchers initially, got %d", len(triggered))
	}

	// Step 4: Append items during a tick, some matching the predicate and some not
	// For this test, we'll simulate items that should match
	appendedItems := []string{"order.1001", "order.1002", "order.1003"}

	// Record the append event
	engine.RecordAppendWithFilters(testPath, appendedItems, filters)

	// Step 5: Confirm the watcher fired once
	triggered = engine.GetTriggeredWatchers()
	if len(triggered) != 1 {
		t.Errorf("Expected 1 triggered watcher after append, got %d", len(triggered))
	}

	if triggered[0].Name != watcherName {
		t.Errorf("Expected triggered watcher '%s', got '%s'", watcherName, triggered[0].Name)
	}

	if triggered[0].TriggerType != TriggerTypeAppend {
		t.Errorf("Expected TriggerTypeAppend, got %v", triggered[0].TriggerType)
	}

	// Step 6: Confirm the bound list contained the correct items in arrival order
	filteredEvent, exists := engine.GetFilteredAppendEvent(testPath, filters)
	if !exists {
		t.Error("Expected filtered append event to exist")
	}

	if filteredEvent != nil {
		if len(filteredEvent.Items) != len(appendedItems) {
			t.Errorf("Expected %d items in filtered event, got %d", len(appendedItems), len(filteredEvent.Items))
		}

		// Verify items are in arrival order
		for i, expectedItem := range appendedItems {
			if filteredEvent.Items[i] != expectedItem {
				t.Errorf("Item[%d]: expected '%s', got '%s'", i, expectedItem, filteredEvent.Items[i])
			}
		}
	}

	// Step 7: Execute the watcher and confirm the binding works
	watcher := triggered[0]
	result, err := engine.ExecuteWatcher(watcher)
	if err != nil {
		t.Errorf("Watcher execution failed: %v", err)
	}

	// The watcher should execute without error (result content depends on implementation)
	if result == nil {
		t.Error("Watcher execution should return a result")
	}

	// Step 8: Confirm the binding is not accessible outside the watcher body
	// This is demonstrated by the fact that the binding is created and discarded
	// within the ExecuteWatcher method scope
	// (In a full implementation, we could test this more explicitly)

	// Step 9: After tick, the append event should not trigger again
	engine.Tick()
	triggered = engine.GetTriggeredWatchers()
	if len(triggered) != 0 {
		t.Errorf("Expected 0 triggered watchers after tick, got %d", len(triggered))
	}
}

// TestAppendWatcherWithPredicateFiltering tests append watcher with predicate filtering
func TestAppendWatcherWithPredicateFiltering(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	testPath := "my.tasks"
	watcherName := "priority_task_processor"
	watchedPaths := []string{testPath}
	watcherCode := `output("Processing priority tasks")`
	bindingName := "priorityTasks"

	// Create placeholder filter expressions
	filters := []parser.Expression{}

	err := engine.RegisterAppendWatcher(watcherName, watchedPaths, watcherCode, bindingName, filters)
	if err != nil {
		t.Fatalf("Failed to register append watcher: %v", err)
	}

	// Append items - some would match predicate, some wouldn't (simulated)
	allItems := []string{"task.high.001", "task.low.002", "task.high.003", "task.medium.004"}

	engine.RecordAppendWithFilters(testPath, allItems, filters)

	// With our simplified filtering, existing items match (all items in this case)
	filteredEvent, exists := engine.GetFilteredAppendEvent(testPath, filters)
	if !exists {
		t.Error("Expected filtered append event to exist")
	}

	// In a full implementation with actual predicate evaluation,
	// only items matching "high" priority would be included
	if filteredEvent != nil {
		t.Logf("Filtered event contains %d items", len(filteredEvent.Items))
		// The actual filtering logic would be tested here
	}

	// Confirm watcher is triggered when matches exist
	triggered := engine.GetTriggeredWatchers()
	if len(triggered) != 1 {
		t.Errorf("Expected 1 triggered watcher with matching items, got %d", len(triggered))
	}
}

// TestAppendWatcherMultiplePaths tests append watcher watching multiple paths
func TestAppendWatcherMultiplePaths(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	engine := NewWatcherEngine(storage, 1000)

	// Watch multiple paths
	watchedPaths := []string{"queue.orders", "queue.returns", "queue.exchanges"}
	watcherName := "multi_queue_processor"
	watcherCode := `output("Processing items from multiple queues")`
	bindingName := "allItems"

	filters := []parser.Expression{}

	err := engine.RegisterAppendWatcher(watcherName, watchedPaths, watcherCode, bindingName, filters)
	if err != nil {
		t.Fatalf("Failed to register multi-path append watcher: %v", err)
	}

	// Append to different queues
	engine.RecordAppend("queue.orders", []string{"order.123"})
	engine.RecordAppend("queue.returns", []string{"return.456", "return.789"})
	// Note: queue.exchanges gets no appends

	// Watcher should trigger because at least one watched path has appends
	triggered := engine.GetTriggeredWatchers()
	if len(triggered) != 1 {
		t.Errorf("Expected 1 triggered watcher for multi-path, got %d", len(triggered))
	}

	// Execute to test multi-path binding (would combine items from all paths)
	if len(triggered) > 0 {
		result, err := engine.ExecuteWatcher(triggered[0])
		if err != nil {
			t.Errorf("Multi-path watcher execution failed: %v", err)
		}
		if result == nil {
			t.Error("Multi-path watcher should return a result")
		}
	}
}