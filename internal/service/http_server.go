package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/solifugus/amorphdb/internal/config"
	"github.com/solifugus/amorphdb/internal/storage"
	"github.com/solifugus/amorphdb/internal/types"
	"github.com/solifugus/amorphdb/web/boilerplate"
)

// HTTPServer handles HTTP requests for PWA static asset serving
type HTTPServer struct {
	assetCache      *AssetCacheManager
	sseManager      *SSEManager
	pwaBridge       *PWABridge
	server          *http.Server
	httpsServer     *http.Server                     // HTTPS server with TLS
	tlsConfig       *tls.Config                      // TLS configuration with SNI support
	tree            storage.ExtendedTree             // Storage tree for request queue
	agentID         uint64                           // Agent ID for request queue writes
	requestTTL      time.Duration                    // Timeout for external request responses
	pendingRequests map[string]chan *RequestResponse // Pending request tracking
	pendingMutex    sync.RWMutex                     // Mutex for pending requests
}

// RequestResponse represents a response to an external request
type RequestResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
}

// NewHTTPServer creates a new HTTP server with the given asset cache manager, SSE manager, and PWA bridge
func NewHTTPServer(httpAddr, httpsAddr string, tlsConfigs map[string]config.TLSConfig, assetCache *AssetCacheManager, sseManager *SSEManager, pwaBridge *PWABridge, tree storage.ExtendedTree, agentID uint64) *HTTPServer {
	httpServer := &HTTPServer{
		assetCache:      assetCache,
		sseManager:      sseManager,
		pwaBridge:       pwaBridge,
		tree:            tree,
		agentID:         agentID,
		requestTTL:      5 * time.Second, // Default TTL of 5 seconds
		pendingRequests: make(map[string]chan *RequestResponse),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", httpServer.handleRequest)

	// Create HTTP server (redirects to HTTPS)
	httpServer.server = &http.Server{
		Addr:         httpAddr,
		Handler:      http.HandlerFunc(httpServer.redirectToHTTPS),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Create HTTPS server with TLS configuration
	if len(tlsConfigs) > 0 {
		// Set up TLS configuration with SNI support
		tlsConfig := &tls.Config{
			GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				domain := info.ServerName
				if domain == "" {
					// Fallback to first configured domain if no SNI
					for firstDomain := range tlsConfigs {
						domain = firstDomain
						break
					}
				}

				config, exists := tlsConfigs[domain]
				if !exists {
					return nil, fmt.Errorf("no TLS configuration for domain: %s", domain)
				}

				cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
				if err != nil {
					return nil, fmt.Errorf("failed to load certificate for domain %s: %w", domain, err)
				}

				return &cert, nil
			},
		}

		httpServer.tlsConfig = tlsConfig
		httpServer.httpsServer = &http.Server{
			Addr:         httpsAddr,
			Handler:      mux,
			TLSConfig:    tlsConfig,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		}
	}

	return httpServer
}

// redirectToHTTPS handles HTTP requests by redirecting them to HTTPS
func (hs *HTTPServer) redirectToHTTPS(w http.ResponseWriter, r *http.Request) {
	if hs.httpsServer == nil {
		// No HTTPS configured, serve HTTP normally
		hs.handleRequest(w, r)
		return
	}

	// Redirect to HTTPS
	httpsURL := "https://" + r.Host + r.URL.Path
	if r.URL.RawQuery != "" {
		httpsURL += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, httpsURL, http.StatusPermanentRedirect)
}

// Start starts both HTTP and HTTPS servers
func (hs *HTTPServer) Start() error {
	// Start HTTP server in background (for redirects or standalone HTTP)
	go func() {
		if err := hs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error but don't fail startup - HTTPS is primary
		}
	}()

	// Start HTTPS server if configured
	if hs.httpsServer != nil {
		return hs.httpsServer.ListenAndServeTLS("", "") // Certs handled by TLS config
	}

	// If no HTTPS server, wait for HTTP server to finish
	return hs.server.ListenAndServe()
}

// Stop stops both HTTP and HTTPS servers gracefully
func (hs *HTTPServer) Stop() error {
	var httpErr, httpsErr error

	// Stop HTTP server
	if hs.server != nil {
		httpErr = hs.server.Close()
	}

	// Stop HTTPS server
	if hs.httpsServer != nil {
		httpsErr = hs.httpsServer.Close()
	}

	// Return first error encountered
	if httpErr != nil {
		return httpErr
	}
	return httpsErr
}

// handleRequest implements the PWA static asset serving with SPA fallback
// Following the design spec SPA fallback order:
// 1. Exact asset match in cache → serve asset
// 1.5. MCP path → hand off to PWA bridge
// 2. SSE path → hand off to SSE handler
// 3. No match, spa_mode = true → return index.html
// 4. No match, spa_mode = false → 503
func (hs *HTTPServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Extract domain from Host header
	domain := extractDomain(r.Host)
	if domain == "" {
		http.Error(w, "Invalid host", http.StatusBadRequest)
		return
	}

	// Clean the request path
	requestPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if requestPath == "" {
		requestPath = "index.html"
	}

	// Step 0: Serve the embedded PWA client boilerplate. This is served from the
	// binary for every domain so apps can load it with a single
	// <script src="/amorphdb/pwa.js"> tag without bundling or hosting it.
	if requestPath == "amorphdb/pwa.js" {
		hs.servePWABoilerplate(w)
		return
	}

	// Step 1: Check for exact asset match in cache
	asset := hs.assetCache.GetAsset(domain, requestPath)
	if asset != nil {
		hs.serveAsset(w, asset)
		return
	}

	// Step 1.5: MCP path handling
	if hs.pwaBridge != nil && hs.pwaBridge.IsMCPPath(requestPath) {
		hs.pwaBridge.HandleMCPRequest(w, r)
		return
	}

	// Step 1.6: Client SSE path handling (PWA client streams)
	if hs.pwaBridge != nil {
		if isClientSSE, deviceID := hs.pwaBridge.IsClientSSEPath(requestPath); isClientSSE {
			hs.pwaBridge.HandleClientSSERequest(w, r, deviceID)
			return
		}
	}

	// Step 1.7: Subscription path handling
	if hs.pwaBridge != nil && hs.pwaBridge.IsSubscriptionPath(requestPath) {
		hs.pwaBridge.HandleSubscriptionRequest(w, r)
		return
	}

	// Step 2: SSE path handling
	if isSSE, streamName := hs.sseManager.IsSSEPath(requestPath); isSSE {
		hs.sseManager.HandleSSERequest(w, r, streamName)
		return
	}

	// Step 3 & 4: Check PWA configuration for SPA fallback
	enabled, spaMode := hs.assetCache.IsEnabled(domain)
	if !enabled {
		// PWA not enabled for this domain → send to inbound request queue for external API handling
		hs.handleInboundRequest(w, r, domain)
		return
	}

	if spaMode {
		// SPA mode enabled, try to serve index.html
		indexAsset := hs.assetCache.GetAsset(domain, "index.html")
		if indexAsset != nil {
			hs.serveAsset(w, indexAsset)
			return
		}
		// No index.html found, fall through to inbound request queue
	}

	// PWA enabled but either no SPA mode or no index.html available → send to inbound request queue
	hs.handleInboundRequest(w, r, domain)
	return
}

// servePWABoilerplate serves the embedded PWA client framework (amorphdb-pwa.js).
func (hs *HTTPServer) servePWABoilerplate(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	w.Write(boilerplate.PWAJavaScript)
}

// serveAsset serves a static asset with appropriate headers
func (hs *HTTPServer) serveAsset(w http.ResponseWriter, asset *Asset) {
	// Set Content-Type header
	if asset.MimeType != "" {
		w.Header().Set("Content-Type", asset.MimeType)
	} else {
		// Fallback: infer MIME type from path extension
		mimeType := mime.TypeByExtension(path.Ext(asset.Path))
		if mimeType != "" {
			w.Header().Set("Content-Type", mimeType)
		}
	}

	// Set cache headers for static assets
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Last-Modified", asset.LoadTime.UTC().Format(http.TimeFormat))

	// Write asset data
	w.WriteHeader(http.StatusOK)
	w.Write(asset.Data)
}

// extractDomain extracts the domain from a Host header, removing port if present
func extractDomain(host string) string {
	if host == "" {
		return ""
	}

	// Remove port if present
	if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}

	// Basic validation - ensure it looks like a domain
	// Reject domains with consecutive dots or other invalid patterns
	if strings.Contains(host, "..") || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return ""
	}

	// Allow localhost, IP addresses, and valid domain names
	if host == "localhost" || strings.Contains(host, ".") || host == "127.0.0.1" {
		return host
	}

	return ""
}

// handleInboundRequest processes external HTTP requests through the request queue
func (hs *HTTPServer) handleInboundRequest(w http.ResponseWriter, r *http.Request, domain string) {
	// Generate unique request ID (sanitize remote addr to avoid dots in storage keys)
	sanitizedRemoteAddr := strings.ReplaceAll(r.RemoteAddr, ".", "_")
	sanitizedRemoteAddr = strings.ReplaceAll(sanitizedRemoteAddr, ":", "_")
	requestID := fmt.Sprintf("req_%d_%s", time.Now().UnixNano(), sanitizedRemoteAddr)

	// Parse query parameters
	queryParams := make(map[string]interface{})
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			queryParams[key] = values[0] // Take first value for simplicity
		}
	}

	// Parse headers
	headers := make(map[string]interface{})
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[key] = values[0] // Take first value for simplicity
		}
	}

	// Read request body
	bodyBytes := make([]byte, 0)
	if r.Body != nil {
		defer r.Body.Close()
		if bodyData, err := io.ReadAll(r.Body); err == nil {
			bodyBytes = bodyData
		}
	}

	// Extract client IP address
	remoteIP := r.RemoteAddr
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		remoteIP = forwardedFor
	}

	// Create request record
	requestRecord := map[string]interface{}{
		"id":        requestID,
		"domain":    domain,
		"method":    r.Method,
		"path":      r.URL.Path,
		"query":     queryParams,
		"headers":   headers,
		"body":      string(bodyBytes),
		"remote_ip": remoteIP,
		"time":      time.Now().UTC().Format(time.RFC3339),
	}

	// Serialize request record to JSON
	requestJSON, err := json.Marshal(requestRecord)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Write request to storage tree at my.computer.network.web.requests[requestID]
	requestPath := []string{"my", "computer", "network", "web", "requests", requestID}
	requestValue := storage.Value{
		TypeTag: types.TypeRecord,
		Data:    requestJSON,
	}

	if err := hs.tree.Write(requestPath, requestValue, hs.agentID); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Set up response channel for this request
	responseChan := make(chan *RequestResponse, 1)
	hs.pendingMutex.Lock()
	hs.pendingRequests[requestID] = responseChan
	hs.pendingMutex.Unlock()

	// Clean up after we're done
	defer func() {
		hs.pendingMutex.Lock()
		delete(hs.pendingRequests, requestID)
		hs.pendingMutex.Unlock()
		close(responseChan)
	}()

	// Wait for response with TTL timeout
	ctx, cancel := context.WithTimeout(r.Context(), hs.requestTTL)
	defer cancel()

	select {
	case response := <-responseChan:
		// Got a response from a watcher
		if response != nil {
			// Set response headers
			for key, value := range response.Headers {
				w.Header().Set(key, value)
			}

			// Write response
			w.WriteHeader(response.StatusCode)
			w.Write([]byte(response.Body))
		} else {
			// Invalid response
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}

	case <-ctx.Done():
		// TTL expired, return 503
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
	}
}

// SendRequestResponse sends a response for a pending request (called by MBL response helpers)
func (hs *HTTPServer) SendRequestResponse(requestID string, response *RequestResponse) bool {
	hs.pendingMutex.RLock()
	responseChan, exists := hs.pendingRequests[requestID]
	hs.pendingMutex.RUnlock()

	if !exists {
		return false // Request not found or already completed
	}

	select {
	case responseChan <- response:
		return true
	default:
		return false // Channel full or closed
	}
}

// AddHTTPServer adds HTTP server capability to the existing Service
func (s *Service) AddHTTPServer(httpAddr, httpsAddr string, tlsConfigs map[string]config.TLSConfig) (*HTTPServer, error) {
	// Create asset cache manager
	assetCache := NewAssetCacheManager(s.tree, s.watcherEngine, 1) // Agent ID 1 for server operations

	// Create SSE manager
	sseManager := NewSSEManager(s.tree, s.watcherEngine, 1) // Agent ID 1 for server operations

	// Create PWA bridge
	pwaBridge := NewPWABridge(s.tree, sseManager, s.watcherEngine, 1, "app") // Agent ID 1, default app name "app"

	// Create HTTP server
	httpServer := NewHTTPServer(httpAddr, httpsAddr, tlsConfigs, assetCache, sseManager, pwaBridge, s.tree, 1) // Agent ID 1 for server operations

	return httpServer, nil
}
