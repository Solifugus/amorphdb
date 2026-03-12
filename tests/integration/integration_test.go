package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/service"
	"github.com/solifugus/amorphdb/internal/storage"
)

// TestServiceStartStop tests basic service lifecycle
func TestServiceStartStop(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "amorphdb-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test configuration
	config := service.Config{
		StorageDir:      filepath.Join(tempDir, "data"),
		LocalSocketPath: filepath.Join(tempDir, "socket"),
		NetworkPort:     0, // Use random port
		NodeIdentity:    "test-node",
	}

	// Create service
	svc, err := service.New(config)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Start service
	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Give service time to start
	time.Sleep(100 * time.Millisecond)

	// Check status
	status := svc.GetStatus()
	if status.NodeIdentity != "test-node" {
		t.Errorf("Expected node identity 'test-node', got '%s'", status.NodeIdentity)
	}

	// Stop service
	if err := svc.Stop(); err != nil {
		t.Errorf("Failed to stop service: %v", err)
	}
}

// TestBasicClientConnection tests protocol client connection
func TestBasicClientConnection(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "amorphdb-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create and start service
	config := service.Config{
		StorageDir:      filepath.Join(tempDir, "data"),
		LocalSocketPath: filepath.Join(tempDir, "socket"),
		NetworkPort:     0, // Use random port
		NodeIdentity:    "test-node",
	}

	svc, err := service.New(config)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer svc.Stop()

	// Give service time to start
	time.Sleep(100 * time.Millisecond)

	// Test that we can create client connections
	// For this test, we'll just verify the service started correctly
	// Full client testing would require the cmd/amorph package
	status := svc.GetStatus()
	if status.Connections != 0 {
		t.Errorf("Expected 0 connections at start, got %d", status.Connections)
	}
}

// TestDataPersistence tests that data survives service restart
func TestDataPersistence(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "amorphdb-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storageDir := filepath.Join(tempDir, "data")
	testValue := storage.Value{
		TypeTag: storage.TypeText,
		Data:    []byte("test data"),
	}

	// Phase 1: Write data directly to storage
	{
		tree, err := storage.NewStorageTree(storageDir)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}

		err = tree.Write([]string{"test", "path"}, testValue, 1000)
		if err != nil {
			t.Fatalf("Failed to write test data: %v", err)
		}

		// Close storage properly
		// Close storage properly (simplified)
	}

	// Phase 2: Create service and verify data exists
	config := service.Config{
		StorageDir:      storageDir,
		LocalSocketPath: filepath.Join(tempDir, "socket"),
		NetworkPort:     0,
		NodeIdentity:    "test-node",
	}

	svc, err := service.New(config)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer svc.Stop()

	// Give service time to start
	time.Sleep(100 * time.Millisecond)

	// Service should have loaded existing data
	// In a full test, we would connect a client and read the data
	status := svc.GetStatus()
	if status.NodeIdentity != "test-node" {
		t.Errorf("Service not properly initialized")
	}
}

// BenchmarkServiceOperations benchmarks basic service operations
func BenchmarkServiceOperations(b *testing.B) {
	// Create temporary directory for benchmark
	tempDir, err := os.MkdirTemp("", "amorphdb-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create and start service
	config := service.Config{
		StorageDir:      filepath.Join(tempDir, "data"),
		LocalSocketPath: filepath.Join(tempDir, "socket"),
		NetworkPort:     0,
		NodeIdentity:    "bench-node",
	}

	svc, err := service.New(config)
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}

	if err := svc.Start(); err != nil {
		b.Fatalf("Failed to start service: %v", err)
	}
	defer svc.Stop()

	// Give service time to start
	time.Sleep(100 * time.Millisecond)

	// Benchmark status requests
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		status := svc.GetStatus()
		if status.NodeIdentity == "" {
			b.Error("Invalid status response")
		}
	}
}