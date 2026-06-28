package service

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
)

// SSEConnection represents an active SSE connection to a client
type SSEConnection struct {
	StreamName   string                   // Stream name (e.g., "price_updates")
	Writer       http.ResponseWriter      // HTTP response writer
	Flusher      http.Flusher             // HTTP flusher for immediate response
	Done         chan struct{}            // Signal when connection should close
	LastEventID  string                   // Last event ID sent to client
}

// SSEEvent represents an event to be sent via SSE
type SSEEvent struct {
	StreamName string                   // Stream name this event belongs to
	Data       string                   // Event data
	ID         string                   // Event ID
	EventType  string                   // Event type (optional)
	Timestamp  time.Time                // When event was created
}

// SSEManager manages Server-Sent Events connections and data distribution
type SSEManager struct {
	connections     map[string][]*SSEConnection // Active connections by stream name
	eventBuffer     map[string]*SSEEvent        // Latest event per stream (last value wins)
	tree            storage.ExtendedTree        // Storage tree for reading SSE data
	watcher         WatcherEngineInterface      // Watcher engine for change detection
	mutex           sync.RWMutex                // Protects connections and eventBuffer maps
	agentID         uint64                      // Agent ID for storage operations
	lastTickTime    time.Time                   // Last tick processing time
}

// NewSSEManager creates a new SSE connection manager
func NewSSEManager(tree storage.ExtendedTree, watcherEngine WatcherEngineInterface, agentID uint64) *SSEManager {
	manager := &SSEManager{
		connections:  make(map[string][]*SSEConnection),
		eventBuffer:  make(map[string]*SSEEvent),
		tree:         tree,
		watcher:      watcherEngine,
		agentID:      agentID,
		lastTickTime: time.Now(),
	}

	// Set up watcher for SSE data changes
	manager.setupWatcher()

	return manager
}

// HandleSSERequest handles an incoming SSE connection request
func (ssem *SSEManager) HandleSSERequest(w http.ResponseWriter, r *http.Request, streamName string) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	// Get flusher for immediate response
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Create SSE connection
	connection := &SSEConnection{
		StreamName: streamName,
		Writer:     w,
		Flusher:    flusher,
		Done:       make(chan struct{}),
	}

	// Register connection
	ssem.mutex.Lock()
	if ssem.connections[streamName] == nil {
		ssem.connections[streamName] = make([]*SSEConnection, 0)
	}
	ssem.connections[streamName] = append(ssem.connections[streamName], connection)

	// Send any buffered event for this stream
	if event, exists := ssem.eventBuffer[streamName]; exists {
		ssem.sendEventToConnection(connection, event)
	}
	ssem.mutex.Unlock()

	// Send initial connection success message
	fmt.Fprintf(w, "event: connected\n")
	fmt.Fprintf(w, "data: Connected to stream %s\n\n", streamName)
	flusher.Flush()

	// Wait for connection to close
	select {
	case <-r.Context().Done():
		// Client disconnected
	case <-connection.Done:
		// Server-side close
	}

	// Clean up connection
	ssem.removeConnection(streamName, connection)
}

// IsSSEPath checks if a request path is an SSE path
func (ssem *SSEManager) IsSSEPath(requestPath string) (bool, string) {
	if strings.HasPrefix(requestPath, "sse/") && len(requestPath) > 4 {
		streamName := requestPath[4:] // Remove "sse/" prefix
		return true, streamName
	}
	return false, ""
}

// removeConnection removes a connection from the active connections list
func (ssem *SSEManager) removeConnection(streamName string, connection *SSEConnection) {
	ssem.mutex.Lock()
	defer ssem.mutex.Unlock()

	connections := ssem.connections[streamName]
	if connections == nil {
		return
	}

	// Find and remove the connection
	for i, conn := range connections {
		if conn == connection {
			// Remove by swapping with last element and truncating
			connections[i] = connections[len(connections)-1]
			connections = connections[:len(connections)-1]
			break
		}
	}

	// Update connections list
	if len(connections) == 0 {
		delete(ssem.connections, streamName)
	} else {
		ssem.connections[streamName] = connections
	}
}

// sendEventToConnection sends an SSE event to a specific connection
func (ssem *SSEManager) sendEventToConnection(connection *SSEConnection, event *SSEEvent) {
	if event.ID != "" {
		fmt.Fprintf(connection.Writer, "id: %s\n", event.ID)
		connection.LastEventID = event.ID
	}

	if event.EventType != "" {
		fmt.Fprintf(connection.Writer, "event: %s\n", event.EventType)
	}

	// Send data (may be multi-line)
	dataLines := strings.Split(event.Data, "\n")
	for _, line := range dataLines {
		fmt.Fprintf(connection.Writer, "data: %s\n", line)
	}

	fmt.Fprintf(connection.Writer, "\n")
	connection.Flusher.Flush()
}

// broadcastEvent sends an event to all connections for a stream
func (ssem *SSEManager) broadcastEvent(event *SSEEvent) {
	ssem.mutex.RLock()
	connections := ssem.connections[event.StreamName]
	ssem.mutex.RUnlock()

	if connections == nil {
		return // No connections for this stream
	}

	// Send to all connections
	for _, connection := range connections {
		select {
		case <-connection.Done:
			// Connection already closed, skip
			continue
		default:
			ssem.sendEventToConnection(connection, event)
		}
	}
}

// processSSEDataChange handles changes to SSE paths in storage
func (ssem *SSEManager) processSSEDataChange() {
	// This method is called by the watcher when SSE paths change
	// For now, we implement a simple approach: scan for known SSE stream names

	// Common SSE stream names to check
	streamNames := []string{
		"price_updates", "notifications", "status_updates", "heartbeat",
		"chat_messages", "system_alerts", "user_activity", "metrics",
	}

	currentTime := time.Now()

	for _, streamName := range streamNames {
		// Read the SSE path for this stream
		ssePath := []string{"my", "computer", "network", "web", "sse", streamName}
		value, err := ssem.tree.Read(ssePath)
		if err != nil {
			continue // Stream doesn't exist or no data
		}

		// Convert value to string for event data
		var eventData string
		switch value.TypeTag {
		case types.TypeText:
			eventData = string(value.Data)
		case types.TypeNumber:
			eventData = string(value.Data)
		case types.TypeBoolean:
			if len(value.Data) > 0 && value.Data[0] != 0 {
				eventData = "true"
			} else {
				eventData = "false"
			}
		default:
			// For complex types, use string representation of raw data
			eventData = string(value.Data)
		}

		// Create event with unique ID based on timestamp
		event := &SSEEvent{
			StreamName: streamName,
			Data:       eventData,
			ID:         fmt.Sprintf("%d", currentTime.UnixNano()),
			EventType:  "data",
			Timestamp:  currentTime,
		}

		// Buffer the event (last value wins within a tick)
		ssem.mutex.Lock()
		ssem.eventBuffer[streamName] = event
		ssem.mutex.Unlock()
	}

	// Process buffered events (one event per stream per tick)
	ssem.processBufferedEvents()
}

// processBufferedEvents sends all buffered events and clears the buffer
func (ssem *SSEManager) processBufferedEvents() {
	ssem.mutex.Lock()
	eventsToSend := make([]*SSEEvent, 0, len(ssem.eventBuffer))
	for _, event := range ssem.eventBuffer {
		eventsToSend = append(eventsToSend, event)
	}
	// Clear buffer after copying events
	ssem.eventBuffer = make(map[string]*SSEEvent)
	ssem.lastTickTime = time.Now()
	ssem.mutex.Unlock()

	// Send events outside the lock to avoid deadlock
	for _, event := range eventsToSend {
		ssem.broadcastEvent(event)
	}
}

// setupWatcher configures the watcher to monitor SSE path changes
func (ssem *SSEManager) setupWatcher() {
	// Create a watcher for SSE changes
	// This monitors the entire my.computer.network.web.sse tree
	watcherCode := `
		// SSE Data Change Watcher
		// This watcher triggers SSE events when data changes in SSE paths
		println("SSE data change detected")
	`

	// Register the watcher using the correct API
	err := ssem.watcher.RegisterWatcher("sse-data-change", []string{"my.computer.network.web.sse"}, watcherCode)
	if err != nil {
		// Log error but don't fail - SSE will still work with manual refresh
		fmt.Printf("Warning: Could not register SSE data change watcher: %v\n", err)
	}

	// Set up periodic tick processing
	go ssem.periodicTick()
}

// periodicTick periodically processes SSE data changes
func (ssem *SSEManager) periodicTick() {
	ticker := time.NewTicker(1 * time.Second) // Check every second (heartbeat tick simulation)
	defer ticker.Stop()

	for range ticker.C {
		ssem.processSSEDataChange()
	}
}

// GetConnectionCount returns the number of active connections for testing
func (ssem *SSEManager) GetConnectionCount(streamName string) int {
	ssem.mutex.RLock()
	defer ssem.mutex.RUnlock()

	connections := ssem.connections[streamName]
	if connections == nil {
		return 0
	}
	return len(connections)
}

// ForceProcessEvents forces processing of SSE events for testing
func (ssem *SSEManager) ForceProcessEvents() {
	ssem.processSSEDataChange()
}