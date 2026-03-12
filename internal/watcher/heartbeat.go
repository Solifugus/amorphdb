// Package watcher implements the heartbeat execution loop for reactive watchers
package watcher

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/solifugus/amorphdb/internal/types"
)

// HeartbeatEngine manages the ⅓ second tick cycle for watcher execution
type HeartbeatEngine struct {
	watcherEngine *WatcherEngine
	tickInterval  time.Duration
	running       bool
	stopChan      chan bool
	mutex         sync.Mutex
	logger        *log.Logger
}

// TickResult represents the result of a heartbeat tick
type TickResult struct {
	TickTime         time.Time
	WatchersFired    int
	WatchersSucceeded int
	WatchersFailed   int
	Errors           []string
	RolledBack       bool
}

// NewHeartbeatEngine creates a new heartbeat execution engine
func NewHeartbeatEngine(watcherEngine *WatcherEngine, logger *log.Logger) *HeartbeatEngine {
	if logger == nil {
		logger = log.New(log.Writer(), "[Heartbeat] ", log.LstdFlags)
	}

	return &HeartbeatEngine{
		watcherEngine: watcherEngine,
		tickInterval:  time.Millisecond * 333, // ⅓ second
		running:       false,
		stopChan:      make(chan bool),
		logger:        logger,
	}
}

// Start begins the heartbeat execution loop
func (he *HeartbeatEngine) Start() error {
	he.mutex.Lock()
	defer he.mutex.Unlock()

	if he.running {
		return fmt.Errorf("heartbeat engine is already running")
	}

	he.running = true
	go he.heartbeatLoop()
	he.logger.Println("Heartbeat engine started")
	return nil
}

// Stop stops the heartbeat execution loop
func (he *HeartbeatEngine) Stop() error {
	he.mutex.Lock()
	defer he.mutex.Unlock()

	if !he.running {
		return fmt.Errorf("heartbeat engine is not running")
	}

	he.running = false
	he.stopChan <- true
	he.logger.Println("Heartbeat engine stopped")
	return nil
}

// IsRunning returns whether the heartbeat engine is currently running
func (he *HeartbeatEngine) IsRunning() bool {
	he.mutex.Lock()
	defer he.mutex.Unlock()
	return he.running
}

// ExecuteSingleTick executes a single heartbeat tick (for testing)
func (he *HeartbeatEngine) ExecuteSingleTick() *TickResult {
	return he.executeTick()
}

// heartbeatLoop is the main execution loop
func (he *HeartbeatEngine) heartbeatLoop() {
	ticker := time.NewTicker(he.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			result := he.executeTick()
			if len(result.Errors) > 0 || result.RolledBack {
				he.logger.Printf("Tick completed with issues: %d watchers fired, %d succeeded, %d failed, rolled back: %v",
					result.WatchersFired, result.WatchersSucceeded, result.WatchersFailed, result.RolledBack)
				for _, err := range result.Errors {
					he.logger.Printf("  Error: %s", err)
				}
			}
		case <-he.stopChan:
			he.logger.Println("Heartbeat loop terminated")
			return
		}
	}
}

// executeTick executes a single heartbeat tick with atomic semantics
func (he *HeartbeatEngine) executeTick() *TickResult {
	tickTime := time.Now()
	result := &TickResult{
		TickTime: tickTime,
		Errors:   []string{},
	}

	// Get all triggered watchers
	triggeredWatchers := he.watcherEngine.GetTriggeredWatchers()
	result.WatchersFired = len(triggeredWatchers)

	if len(triggeredWatchers) == 0 {
		// Update tick time and clear change log
		he.watcherEngine.Tick()
		he.watcherEngine.ClearChangeLog()
		return result
	}

	// Execute all triggered watchers
	var watcherResults []interface{}
	var watcherErrors []error
	hasUnhandledUnknown := false

	for _, watcher := range triggeredWatchers {
		watcherResult, err := he.watcherEngine.ExecuteWatcher(watcher)
		watcherResults = append(watcherResults, watcherResult)
		watcherErrors = append(watcherErrors, err)

		if err != nil {
			result.WatchersFailed++
			result.Errors = append(result.Errors,
				fmt.Sprintf("Watcher '%s' execution error: %v", watcher.Name, err))
		} else {
			// Check if result is an unhandled Unknown
			if unknown, ok := watcherResult.(types.Unknown); ok {
				// For now, treat all Unknown as unhandled
				// In a full implementation, we'd distinguish between handled and unhandled
				hasUnhandledUnknown = true
				result.Errors = append(result.Errors,
					fmt.Sprintf("Watcher '%s' returned unhandled Unknown: %s", watcher.Name, unknown.Reason))
			}
			result.WatchersSucceeded++
		}
	}

	// Determine if rollback is needed
	if hasUnhandledUnknown {
		// Rollback: discard any staged writes and mark as rolled back
		if coordinator := he.watcherEngine.GetCoordinator(); coordinator != nil {
			coordinator.DiscardAll()
		}
		result.RolledBack = true
		result.Errors = append(result.Errors, "Tick rolled back due to unhandled Unknown values")

		// Don't update tick time - this allows the same changes to trigger again
		return result
	}

	// Commit: flush coordinator if available, then update tick time and clear change log
	if coordinator := he.watcherEngine.GetCoordinator(); coordinator != nil {
		if err := coordinator.FlushAll(); err != nil {
			// If coordinator flush fails, treat as rollback
			coordinator.DiscardAll()
			result.RolledBack = true
			result.Errors = append(result.Errors, fmt.Sprintf("Coordinator flush failed: %v", err))
			return result
		}
	}

	he.watcherEngine.Tick()
	he.watcherEngine.ClearChangeLog()

	return result
}

// SetTickInterval changes the heartbeat interval (for testing)
func (he *HeartbeatEngine) SetTickInterval(interval time.Duration) {
	he.mutex.Lock()
	defer he.mutex.Unlock()
	he.tickInterval = interval
}

// GetTickInterval returns the current tick interval
func (he *HeartbeatEngine) GetTickInterval() time.Duration {
	he.mutex.Lock()
	defer he.mutex.Unlock()
	return he.tickInterval
}

// GetStats returns current heartbeat statistics
func (he *HeartbeatEngine) GetStats() map[string]interface{} {
	he.mutex.Lock()
	defer he.mutex.Unlock()

	return map[string]interface{}{
		"running":      he.running,
		"tick_interval": he.tickInterval.String(),
		"watchers":     len(he.watcherEngine.GetWatchers()),
	}
}