package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/solifugus/amorphdb/internal/config"
	"github.com/solifugus/amorphdb/internal/mbl/interpreter"
	"github.com/solifugus/amorphdb/internal/mesh"
	"github.com/solifugus/amorphdb/internal/protocol"
	"github.com/solifugus/amorphdb/internal/security"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/watcher"
)

// Service represents the AmorphDB service daemon
type Service struct {
	// Storage and execution
	tree          storage.ExtendedTree          // Single shared storage
	permEvaluator *security.PermissionEvaluator // Permission checking
	stampManager  *security.StampManager        // Automatic stamp injection
	filterManager *security.FilterManager       // Response filtering
	watcherEngine *watcher.WatcherEngine        // Reactive programming

	// Mesh management
	meshService *MeshService // Mesh operations

	// Web (PWA / inbound HTTP) serving
	httpServer *HTTPServer                 // HTTP/HTTPS server (nil when disabled)
	httpPort   int                         // HTTP port (0 = disabled)
	httpsPort  int                         // HTTPS port (0 = disabled)
	tlsConfigs map[string]config.TLSConfig // Per-domain TLS configuration

	// Network configuration
	localSocket     net.Listener // UNIX domain socket
	networkSocket   net.Listener // TCP socket (port 5000)
	localSocketPath string       // Path to UNIX socket
	networkPort     int          // Network port number

	// Connection management
	connections      map[string]*Connection // Active client connections
	connectionsMutex sync.RWMutex           // Protects connections map
	nextConnID       uint64                 // Connection ID counter

	// Service state
	startTime    time.Time          // Service start time
	nodeIdentity string             // Unique node identifier
	storageDir   string             // Data storage directory
	ctx          context.Context    // Service context
	cancel       context.CancelFunc // Service cancellation
	shutdownWG   sync.WaitGroup     // Graceful shutdown coordination
}

// Config represents service configuration
type Config struct {
	StorageDir      string // Data storage directory
	LocalSocketPath string // UNIX socket path
	NetworkPort     int    // TCP port
	NodeIdentity    string // Node identifier
	MeshName        string // Mesh name (empty = standalone)
	ConfigPath      string // Configuration file path

	// Web serving (PWA + inbound HTTP). Zero ports disable the HTTP server,
	// which is why tests that only need socket access leave these unset.
	HTTPPort  int                         // HTTP port (0 = disabled)
	HTTPSPort int                         // HTTPS port (0 = disabled)
	TLS       map[string]config.TLSConfig // Per-domain TLS configuration
}

// DefaultConfig returns default service configuration
func DefaultConfig() Config {
	homeDir, _ := os.UserHomeDir()
	amorphDir := filepath.Join(homeDir, ".amorph")

	return Config{
		StorageDir:      filepath.Join(amorphDir, "data"),
		LocalSocketPath: filepath.Join(amorphDir, "socket"),
		NetworkPort:     5000,
		NodeIdentity:    generateNodeIdentity(),
		MeshName:        "", // Empty = standalone mode
		ConfigPath:      filepath.Join(amorphDir, "svcConfig.yaml"),
	}
}

// New creates a new service instance with the given configuration
func New(svcConfig Config) (*Service, error) {
	// Create service context
	ctx, cancel := context.WithCancel(context.Background())

	// Ensure storage directory exists
	if err := os.MkdirAll(svcConfig.StorageDir, 0755); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Initialize storage tree
	tree, err := storage.NewStorageTree(svcConfig.StorageDir)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Create extended tree interface for security components
	extendedTree := storage.NewTreeAdapter(tree)

	// Initialize security components
	permEvaluator := security.NewPermissionEvaluator(extendedTree)
	stampManager := security.NewStampManager(extendedTree)
	filterManager := security.NewFilterManager(extendedTree, permEvaluator)

	// Initialize watcher engine
	watcherEngine := watcher.NewWatcherEngine(extendedTree, 1000)

	// Convert service config to mesh config format
	meshConfig := &config.Config{
		Mesh: config.MeshConfig{
			Name:     svcConfig.MeshName,
			Identity: svcConfig.NodeIdentity,
			Bridges:  make(map[string]string),
		},
		Data: config.DataConfig{
			StorageDir: svcConfig.StorageDir,
		},
		Network: config.NetworkConfig{
			LocalSocketPath: svcConfig.LocalSocketPath,
			Port:            svcConfig.NetworkPort,
		},
	}

	// Initialize mesh manager and service
	meshManager, err := mesh.NewMeshManager(meshConfig, tree, svcConfig.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create mesh manager: %w", err)
	}
	meshService := NewMeshService(meshManager)

	// Create service instance
	service := &Service{
		tree:            extendedTree,
		permEvaluator:   permEvaluator,
		stampManager:    stampManager,
		filterManager:   filterManager,
		watcherEngine:   watcherEngine,
		meshService:     meshService,
		httpPort:        svcConfig.HTTPPort,
		httpsPort:       svcConfig.HTTPSPort,
		tlsConfigs:      svcConfig.TLS,
		localSocketPath: svcConfig.LocalSocketPath,
		networkPort:     svcConfig.NetworkPort,
		connections:     make(map[string]*Connection),
		startTime:       time.Now(),
		nodeIdentity:    svcConfig.NodeIdentity,
		storageDir:      svcConfig.StorageDir,
		ctx:             ctx,
		cancel:          cancel,
	}

	return service, nil
}

// Start starts the service and begins accepting connections
func (s *Service) Start() error {
	// Remove existing socket file if it exists
	if err := os.Remove(s.localSocketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Ensure socket directory exists
	socketDir := filepath.Dir(s.localSocketPath)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Start local UNIX socket listener
	localSocket, err := net.Listen("unix", s.localSocketPath)
	if err != nil {
		return fmt.Errorf("failed to start local socket: %w", err)
	}
	s.localSocket = localSocket

	// Start network TCP socket listener
	networkSocket, err := net.Listen("tcp", fmt.Sprintf(":%d", s.networkPort))
	if err != nil {
		localSocket.Close()
		return fmt.Errorf("failed to start network socket: %w", err)
	}
	s.networkSocket = networkSocket

	// Start watcher engine
	if err := s.watcherEngine.Start(); err != nil {
		localSocket.Close()
		networkSocket.Close()
		return fmt.Errorf("failed to start watcher engine: %w", err)
	}

	// Start connection handlers
	s.shutdownWG.Add(2)
	go s.acceptLocalConnections()
	go s.acceptNetworkConnections()

	// Start the web (PWA / inbound HTTP) server when enabled. Failure here is
	// non-fatal: the socket-based service stays up so amorphctl/amorph keep
	// working even if the web port is unavailable.
	if s.httpPort > 0 || s.httpsPort > 0 {
		if err := s.startWebServer(); err != nil {
			fmt.Printf("Warning: web server not started: %v\n", err)
		}
	}

	fmt.Printf("AmorphDB service started\n")
	fmt.Printf("Node Identity: %s\n", s.nodeIdentity)
	fmt.Printf("Local Socket: %s\n", s.localSocketPath)
	fmt.Printf("Network Port: %d\n", s.networkPort)
	if s.httpServer != nil {
		fmt.Printf("HTTP Port: %d  HTTPS Port: %d\n", s.httpPort, s.httpsPort)
	}
	fmt.Printf("Storage: %s\n", s.storageDir)

	return nil
}

// startWebServer creates and launches the HTTP/HTTPS server for PWA asset
// serving, the PWA bridge, SSE, and the inbound request queue.
func (s *Service) startWebServer() error {
	httpAddr := ""
	if s.httpPort > 0 {
		httpAddr = fmt.Sprintf(":%d", s.httpPort)
	}
	httpsAddr := ""
	if s.httpsPort > 0 {
		httpsAddr = fmt.Sprintf(":%d", s.httpsPort)
	}

	httpServer, err := s.AddHTTPServer(httpAddr, httpsAddr, s.tlsConfigs)
	if err != nil {
		return err
	}
	s.httpServer = httpServer

	s.shutdownWG.Add(1)
	go func() {
		defer s.shutdownWG.Done()
		if err := httpServer.Start(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Web server stopped: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the service gracefully
func (s *Service) Stop() error {
	fmt.Println("Stopping AmorphDB service...")

	// Cancel service context
	s.cancel()

	// Stop the web server (unblocks its goroutine via ErrServerClosed)
	if s.httpServer != nil {
		s.httpServer.Stop()
	}

	// Close listeners
	if s.localSocket != nil {
		s.localSocket.Close()
	}
	if s.networkSocket != nil {
		s.networkSocket.Close()
	}

	// Close all connections
	s.connectionsMutex.Lock()
	for _, conn := range s.connections {
		conn.Close()
	}
	s.connectionsMutex.Unlock()

	// Stop watcher engine
	if s.watcherEngine != nil {
		s.watcherEngine.Stop()
	}

	// Wait for all goroutines to finish
	s.shutdownWG.Wait()

	// Clean up socket file
	if s.localSocketPath != "" {
		os.Remove(s.localSocketPath)
	}

	fmt.Println("AmorphDB service stopped")
	return nil
}

// Wait waits for the service to shut down
func (s *Service) Wait() {
	<-s.ctx.Done()
}

// GetStatus returns current service status information
func (s *Service) GetStatus() protocol.StatusResponseMessage {
	s.connectionsMutex.RLock()
	connectionCount := len(s.connections)
	s.connectionsMutex.RUnlock()

	uptime := time.Since(s.startTime).Seconds()

	// Get current node identity (may have been updated by mesh operations)
	currentNodeIdentity := s.nodeIdentity
	if s.meshService != nil && !s.meshService.IsStandalone() {
		currentNodeIdentity = s.meshService.GetNodeIdentity()
	}

	return protocol.StatusResponseMessage{
		Uptime:       int64(uptime),
		NodeIdentity: currentNodeIdentity,
		LocalSocket:  s.localSocketPath,
		NetworkPort:  s.networkPort,
		Connections:  connectionCount,
		DataSize:     s.getDataSize(),
	}
}

// acceptLocalConnections handles incoming local socket connections
func (s *Service) acceptLocalConnections() {
	defer s.shutdownWG.Done()

	for {
		conn, err := s.localSocket.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return // Service is shutting down
			default:
				fmt.Printf("Local socket accept error: %v\n", err)
				continue
			}
		}

		// Handle connection in a new goroutine
		go s.handleConnection(conn, true) // true = local connection
	}
}

// acceptNetworkConnections handles incoming network connections
func (s *Service) acceptNetworkConnections() {
	defer s.shutdownWG.Done()

	for {
		conn, err := s.networkSocket.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return // Service is shutting down
			default:
				fmt.Printf("Network socket accept error: %v\n", err)
				continue
			}
		}

		// Handle connection in a new goroutine
		go s.handleConnection(conn, false) // false = network connection
	}
}

// handleConnection processes a new client connection
func (s *Service) handleConnection(netConn net.Conn, isLocal bool) {
	// Generate connection ID
	s.connectionsMutex.Lock()
	connID := fmt.Sprintf("conn-%d", s.nextConnID)
	s.nextConnID++
	s.connectionsMutex.Unlock()

	// Create connection wrapper
	conn := NewConnection(connID, netConn, isLocal, s)

	// Register connection
	s.connectionsMutex.Lock()
	s.connections[connID] = conn
	s.connectionsMutex.Unlock()

	// Start connection handler
	conn.Start()

	// Clean up when connection ends
	s.connectionsMutex.Lock()
	delete(s.connections, connID)
	s.connectionsMutex.Unlock()
}

// getDataSize estimates the total data size (placeholder implementation)
func (s *Service) getDataSize() int64 {
	// In a real implementation, this would calculate the actual storage size
	// For now, return a placeholder value
	return 1048576 // 1MB placeholder
}

// GetLocalSocketPath returns the local socket path (for testing)
func (s *Service) GetLocalSocketPath() string {
	return s.localSocketPath
}

// GetInterpreter creates a new interpreter instance for the given agent (for testing)
func (s *Service) GetInterpreter(agentID uint64) *interpreter.Interpreter {
	return interpreter.New(s.tree, agentID)
}

// GetTree returns the storage tree (for testing)
func (s *Service) GetTree() storage.Tree {
	return s.tree
}

// generateNodeIdentity generates a unique node identifier
func generateNodeIdentity() string {
	// Use hostname + timestamp for uniqueness
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s-%d", hostname, timestamp)
}
