// Package watcher implements heartbeat execution loop tests
package watcher

import (
	"log"
	"os"
	"testing"
	"time"
)

// TestHeartbeatEngineBasics tests basic heartbeat engine functionality
func TestHeartbeatEngineBasics(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	watcherEngine := NewWatcherEngine(storage, 1000)
	logger := log.New(os.Stdout, "[Test] ", log.LstdFlags)
	heartbeat := NewHeartbeatEngine(watcherEngine, logger)

	// Initially not running
	if heartbeat.IsRunning() {
		t.Error("Heartbeat should not be running initially")
	}

	// Test starting
	err := heartbeat.Start()
	if err != nil {
		t.Errorf("Failed to start heartbeat: %v", err)
	}

	if !heartbeat.IsRunning() {
		t.Error("Heartbeat should be running after start")
	}

	// Test double start (should fail)
	err = heartbeat.Start()
	if err == nil {
		t.Error("Starting heartbeat twice should return an error")
	}

	// Test stopping
	err = heartbeat.Stop()
	if err != nil {
		t.Errorf("Failed to stop heartbeat: %v", err)
	}

	if heartbeat.IsRunning() {
		t.Error("Heartbeat should not be running after stop")
	}

	// Test double stop (should fail)
	err = heartbeat.Stop()
	if err == nil {
		t.Error("Stopping heartbeat twice should return an error")
	}
}

// TestHeartbeatSingleTick tests executing a single tick
func TestHeartbeatSingleTick(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	watcherEngine := NewWatcherEngine(storage, 1000)
	heartbeat := NewHeartbeatEngine(watcherEngine, nil)

	// Execute tick with no watchers
	result := heartbeat.ExecuteSingleTick()
	if result.WatchersFired != 0 {
		t.Errorf("Expected 0 watchers fired, got %d", result.WatchersFired)
	}

	if result.RolledBack {
		t.Error("Tick should not be rolled back with no watchers")
	}

	if len(result.Errors) != 0 {
		t.Errorf("Expected no errors, got %v", result.Errors)
	}
}

// TestHeartbeatWithWatchers tests heartbeat execution with watchers
func TestHeartbeatWithWatchers(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	watcherEngine := NewWatcherEngine(storage, 1000)
	heartbeat := NewHeartbeatEngine(watcherEngine, nil)

	// Register a successful watcher
	err := watcherEngine.RegisterWatcher("success_watcher", []string{"my.data"}, "42")
	if err != nil {
		t.Fatalf("Failed to register watcher: %v", err)
	}

	// Record a change to trigger the watcher
	watcherEngine.RecordChange("my.data")

	// Execute a tick
	result := heartbeat.ExecuteSingleTick()

	if result.WatchersFired != 1 {
		t.Errorf("Expected 1 watcher fired, got %d", result.WatchersFired)
	}

	if result.WatchersSucceeded != 1 {
		t.Errorf("Expected 1 watcher succeeded, got %d", result.WatchersSucceeded)
	}

	if result.WatchersFailed != 0 {
		t.Errorf("Expected 0 watchers failed, got %d", result.WatchersFailed)
	}

	if result.RolledBack {
		t.Error("Successful tick should not be rolled back")
	}

	// After tick, the same change should not trigger again
	result = heartbeat.ExecuteSingleTick()
	if result.WatchersFired != 0 {
		t.Errorf("Expected 0 watchers fired after tick, got %d", result.WatchersFired)
	}
}

// TestHeartbeatWithUnknown tests heartbeat rollback on Unknown values
func TestHeartbeatWithUnknown(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	watcherEngine := NewWatcherEngine(storage, 1000)
	heartbeat := NewHeartbeatEngine(watcherEngine, nil)

	// Register a watcher that produces Unknown
	err := watcherEngine.RegisterWatcher("unknown_watcher", []string{"my.error"}, "undefined_variable")
	if err != nil {
		t.Fatalf("Failed to register watcher: %v", err)
	}

	// Record a change to trigger the watcher
	watcherEngine.RecordChange("my.error")

	// Execute a tick
	result := heartbeat.ExecuteSingleTick()

	if result.WatchersFired != 1 {
		t.Errorf("Expected 1 watcher fired, got %d", result.WatchersFired)
	}

	if result.WatchersSucceeded != 1 {
		t.Errorf("Expected 1 watcher succeeded (Unknown is not an error), got %d", result.WatchersSucceeded)
	}

	if !result.RolledBack {
		t.Error("Tick should be rolled back due to Unknown")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors due to rollback")
	}

	// After rollback, the same change should trigger again
	result = heartbeat.ExecuteSingleTick()
	if result.WatchersFired != 1 {
		t.Errorf("Expected 1 watcher fired after rollback, got %d", result.WatchersFired)
	}
}

// TestHeartbeatWithMultipleWatchers tests multiple watchers in one tick
func TestHeartbeatWithMultipleWatchers(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	watcherEngine := NewWatcherEngine(storage, 1000)
	heartbeat := NewHeartbeatEngine(watcherEngine, nil)

	// Register multiple watchers
	err := watcherEngine.RegisterWatcher("watcher1", []string{"path1"}, "1")
	if err != nil {
		t.Fatalf("Failed to register watcher1: %v", err)
	}

	err = watcherEngine.RegisterWatcher("watcher2", []string{"path2"}, "2")
	if err != nil {
		t.Fatalf("Failed to register watcher2: %v", err)
	}

	err = watcherEngine.RegisterWatcher("watcher3", []string{"path1"}, "3")
	if err != nil {
		t.Fatalf("Failed to register watcher3: %v", err)
	}

	// Trigger path1 (should fire watcher1 and watcher3)
	watcherEngine.RecordChange("path1")

	// Execute a tick
	result := heartbeat.ExecuteSingleTick()

	if result.WatchersFired != 2 {
		t.Errorf("Expected 2 watchers fired, got %d", result.WatchersFired)
	}

	if result.WatchersSucceeded != 2 {
		t.Errorf("Expected 2 watchers succeeded, got %d", result.WatchersSucceeded)
	}

	if result.WatchersFailed != 0 {
		t.Errorf("Expected 0 watchers failed, got %d", result.WatchersFailed)
	}
}

// TestHeartbeatTickInterval tests changing tick interval
func TestHeartbeatTickInterval(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	watcherEngine := NewWatcherEngine(storage, 1000)
	heartbeat := NewHeartbeatEngine(watcherEngine, nil)

	// Default interval should be 333ms (⅓ second)
	defaultInterval := heartbeat.GetTickInterval()
	expectedDefault := time.Millisecond * 333
	if defaultInterval != expectedDefault {
		t.Errorf("Expected default interval %v, got %v", expectedDefault, defaultInterval)
	}

	// Change interval
	newInterval := time.Millisecond * 100
	heartbeat.SetTickInterval(newInterval)

	if heartbeat.GetTickInterval() != newInterval {
		t.Errorf("Expected new interval %v, got %v", newInterval, heartbeat.GetTickInterval())
	}
}

// TestHeartbeatStats tests getting heartbeat statistics
func TestHeartbeatStats(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	watcherEngine := NewWatcherEngine(storage, 1000)

	// Register a watcher
	err := watcherEngine.RegisterWatcher("test_watcher", []string{"test.path"}, "42")
	if err != nil {
		t.Fatalf("Failed to register watcher: %v", err)
	}

	heartbeat := NewHeartbeatEngine(watcherEngine, nil)

	stats := heartbeat.GetStats()

	// Check that stats contains expected keys
	if running, ok := stats["running"].(bool); !ok || running {
		t.Error("Expected running to be false")
	}

	if tickInterval, ok := stats["tick_interval"].(string); !ok || tickInterval == "" {
		t.Error("Expected tick_interval to be a non-empty string")
	}

	if watchers, ok := stats["watchers"].(int); !ok || watchers != 1 {
		t.Errorf("Expected watchers to be 1, got %v", watchers)
	}
}

// TestHeartbeatIntegration tests the full heartbeat integration with short intervals
func TestHeartbeatIntegration(t *testing.T) {
	storage := createTestStorage(t)
	defer storage.Close()

	watcherEngine := NewWatcherEngine(storage, 1000)
	heartbeat := NewHeartbeatEngine(watcherEngine, nil)

	// Set a very short interval for testing
	heartbeat.SetTickInterval(time.Millisecond * 10)

	// Register a watcher
	err := watcherEngine.RegisterWatcher("integration_watcher", []string{"integration.test"}, "100")
	if err != nil {
		t.Fatalf("Failed to register watcher: %v", err)
	}

	// Start heartbeat
	err = heartbeat.Start()
	if err != nil {
		t.Fatalf("Failed to start heartbeat: %v", err)
	}
	defer heartbeat.Stop()

	// Trigger a change
	watcherEngine.RecordChange("integration.test")

	// Wait a bit for heartbeat to process
	time.Sleep(time.Millisecond * 50)

	// The change should have been processed and cleared
	triggered := watcherEngine.GetTriggeredWatchers()
	if len(triggered) != 0 {
		t.Errorf("Expected no triggered watchers after heartbeat processing, got %d", len(triggered))
	}
}