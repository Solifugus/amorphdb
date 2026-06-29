package service

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/solifugus/amorphdb/web/boilerplate"
)

// freeTCPPort returns a currently-free localhost TCP port for the daemon to
// bind, avoiding the fixed-port clashes that plague the rest of the suite.
func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not reserve a free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// TestService_ServesPWAOverHTTP is the end-to-end guard for the daemon wiring:
// a Service started with an HTTP port must actually listen and serve the
// embedded PWA client at /amorphdb/pwa.js. Before the web server was wired into
// Service.Start(), AddHTTPServer was dead code and nothing listened on HTTP.
func TestService_ServesPWAOverHTTP(t *testing.T) {
	dir := t.TempDir()
	httpPort := freeTCPPort(t)

	svc, err := New(Config{
		StorageDir:      filepath.Join(dir, "data"),
		LocalSocketPath: filepath.Join(dir, "sock"),
		NetworkPort:     0, // ephemeral — don't collide with other tests' :5000
		NodeIdentity:    "web-serve-test",
		HTTPPort:        httpPort,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer svc.Stop()

	url := fmt.Sprintf("http://127.0.0.1:%d/amorphdb/pwa.js", httpPort)

	// The HTTP listener comes up asynchronously; poll briefly until ready.
	var resp *http.Response
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err = http.Get(url)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("HTTP server never became ready: %v", err)
		}
		time.Sleep(25 * time.Millisecond)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", url, resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
		t.Errorf("Content-Type = %q, want application/javascript", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if len(body) != len(boilerplate.PWAJavaScript) {
		t.Errorf("served %d bytes, embedded boilerplate is %d", len(body), len(boilerplate.PWAJavaScript))
	}
}

// TestService_WebServerDisabledByDefault verifies that a Service configured
// without web ports (the shape used throughout the test suite) starts no HTTP
// listener, so enabling web serving never silently binds extra ports in tests.
func TestService_WebServerDisabledByDefault(t *testing.T) {
	dir := t.TempDir()

	svc, err := New(Config{
		StorageDir:      filepath.Join(dir, "data"),
		LocalSocketPath: filepath.Join(dir, "sock"),
		NetworkPort:     0,
		NodeIdentity:    "no-web-test",
		// HTTPPort / HTTPSPort intentionally left 0.
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer svc.Stop()

	if svc.httpServer != nil {
		t.Errorf("expected no HTTP server when ports are 0, got %v", svc.httpServer)
	}
}
