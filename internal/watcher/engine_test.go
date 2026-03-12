// Package watcher implements MBL watchers and the reactive execution engine
package watcher

import (
	"os"
	"path/filepath"
	"testing"

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