# AmorphDB Bridge Architecture

## Overview

AmorphDB's bridge architecture enables seamless multi-mesh connectivity through mobile agent identities and cross-mesh data access patterns. This document details the technical implementation of the bridging system.

## Architecture Components

### 1. Identity Management System

#### Node vs Mobile Agent Identities

```go
type AgentType int
const (
    NodeAgent   AgentType = iota  // Permanent mesh member
    MobileAgent                   // Bridge identity in partner mesh
)

type Identity struct {
    ID       string    `json:"id"`        // Unique identifier
    Type     AgentType `json:"type"`      // Agent type
    MeshName string    `json:"mesh_name"` // Owning mesh
}
```

**Node Agent:**
- Permanent member of exactly one mesh
- Has local storage and zone responsibilities
- Participates in mesh consensus and gossip protocols

**Mobile Agent:**
- Temporary identity for bridge connections
- No local storage in partner mesh
- Data stored under `world.agent.{identity_id}`

#### Identity Generation

```go
func (im *IdentityManager) CreateBridgeIdentity(targetMesh string) (*Identity, error) {
    return &Identity{
        ID:       generateBridgeIdentityID(),
        Type:     MobileAgent,
        MeshName: targetMesh,
    }, nil
}
```

Bridge identities are:
- Generated using cryptographically secure random IDs
- Unique across all meshes
- Linked to the originating node's identity
- Stored in both local mesh and partner mesh

### 2. Bridge Connection Management

#### Bridge Manager Architecture

```go
type BridgeManager struct {
    primaryMesh    string                        // Local mesh name
    bridges        map[string]*BridgeConnection  // mesh_name -> connection
    identities     map[string]*Identity          // mesh_name -> bridge identity
    authSessions   map[string]*AuthSession       // session management
}

type BridgeConnection struct {
    meshName       string          // Target mesh name
    address        string          // Connection endpoint
    status         ConnectionStatus // Connection state
    identity       *Identity       // Bridge identity in target mesh
    lastHeartbeat  time.Time       // Connection health
    dataCache      *BridgeCache    // Local data cache
}
```

#### Connection Lifecycle

1. **Discovery Phase:**
   ```go
   func (bm *BridgeManager) CreateBridge(targetAddress string) error {
       // 1. Connect to target mesh
       conn, err := bm.connectToTarget(targetAddress)

       // 2. Perform mesh discovery
       meshInfo, err := bm.discoverMeshInfo(conn)

       // 3. Request bridge identity assignment
       identity, err := bm.requestBridgeIdentity(meshInfo)

       // 4. Establish authenticated connection
       return bm.establishBridge(meshInfo, identity)
   }
   ```

2. **Authentication Phase:**
   - Mobile agent key derivation
   - Challenge-response protocol
   - Session establishment

3. **Operation Phase:**
   - Data access and caching
   - Periodic authentication refresh
   - Connection health monitoring

4. **Termination Phase:**
   - Graceful disconnection
   - Identity cleanup
   - Cache invalidation

### 3. Data Access Architecture

#### Path Resolution System

Bridge data access uses the `my.{mesh-name}.*` pattern:

```go
type BridgeScope struct {
    meshName     string  // Target mesh name
    agentHome    string  // world.agent.{bridge_identity}
    localPath    string  // Original my.meshname.* path
    connection   *BridgeConnection
}

func (br *BridgeScopeResolver) ResolveBridgePath(path string) (*BridgeScope, error) {
    if !strings.HasPrefix(path, "my.") {
        return nil, nil // Not a bridge path
    }

    parts := strings.Split(path, ".")
    if len(parts) < 2 {
        return nil, ErrInvalidPath
    }

    meshName := parts[1]
    bridge := br.bridges[meshName]
    if bridge == nil {
        return nil, ErrBridgeNotFound
    }

    return &BridgeScope{
        meshName:  meshName,
        agentHome: fmt.Sprintf("world.agent.%s", bridge.identity.ID),
        localPath: path,
        connection: bridge,
    }, nil
}
```

#### Data Translation

**Local Access Pattern:**
```
my.partner-mesh.projects.alpha.config
```

**Remote Storage Location:**
```
world.agent.{bridge-identity-id}.projects.alpha.config
```

**Implementation:**
```go
func (bs *BridgeStorage) WriteBridgeData(components []string, value Value, author uint64) error {
    // Transform path: my.mesh.path -> world.agent.{id}.path
    remotePath := bs.transformBridgePath(components)

    // Write to remote mesh via bridge connection
    return bs.writeToRemoteMesh(remotePath, value, author)
}

func (bs *BridgeStorage) transformBridgePath(components []string) []string {
    // Input: ["my", "partner-mesh", "projects", "alpha", "config"]
    // Output: ["world", "agent", "{bridge-id}", "projects", "alpha", "config"]

    if len(components) < 3 || components[0] != "my" {
        return components // Not a bridge path
    }

    meshName := components[1]
    bridge := bs.getBridge(meshName)

    result := []string{"world", "agent", bridge.identity.ID}
    result = append(result, components[2:]...) // Skip "my.meshname"
    return result
}
```

### 4. Authentication Architecture

#### Mobile Agent Key Derivation

```go
type MobileKeyDerivation struct {
    passphrase   string  // User-provided passphrase
    deviceSecret string  // Device-specific secret
    nodeID       string  // Local node identifier
}

func (mkd *MobileKeyDerivation) DeriveKeyPairForMesh(meshName string) (*DerivedKey, error) {
    // Combine inputs with mesh-specific salt
    input := fmt.Sprintf("%s:%s:%s:%s", mkd.passphrase, mkd.deviceSecret, mkd.nodeID, meshName)

    // Generate key using PBKDF2
    keyMaterial := pbkdf2.Key([]byte(input), []byte(meshName), 100000, 64, sha256.New)

    return &DerivedKey{
        MeshName:    meshName,
        PrivateKey:  keyMaterial[:32],
        PublicKey:   generatePublicKey(keyMaterial[:32]),
        Timestamp:   time.Now(),
    }, nil
}
```

#### Challenge-Response Protocol

```go
type AuthChallenge struct {
    SessionID      string    `json:"session_id"`
    Challenge      []byte    `json:"challenge"`
    AgentIdentity  string    `json:"agent_identity"`
    Timestamp      time.Time `json:"timestamp"`
}

func (ba *BridgeAuth) CreateChallenge(sessionID string) (*AuthChallenge, error) {
    session := ba.sessions[sessionID]
    if session == nil {
        return nil, ErrSessionNotFound
    }

    challenge := make([]byte, 32)
    rand.Read(challenge)

    session.Challenge = challenge
    session.Status = "challenging"

    return &AuthChallenge{
        SessionID:     sessionID,
        Challenge:     challenge,
        AgentIdentity: session.Identity.ID,
        Timestamp:     time.Now(),
    }, nil
}

func (ba *BridgeAuth) RespondToChallenge(challenge *AuthChallenge, meshName string) ([]byte, error) {
    // Get mobile identity for this mesh
    identity := ba.mobileIdentities[meshName]
    if identity == nil {
        return nil, ErrNoIdentityForMesh
    }

    // Sign challenge with derived key
    key := identity.DerivedKey
    signature := ed25519.Sign(key.PrivateKey, challenge.Challenge)

    return signature, nil
}
```

### 5. Data Caching and Consistency

#### Bridge Cache Architecture

```go
type BridgeCache struct {
    entries    map[string]*CacheEntry  // path -> cached data
    ttl        time.Duration           // Cache time-to-live
    maxSize    int                     // Maximum entries
    stats      CacheStats             // Performance metrics
}

type CacheEntry struct {
    path       string      // Full remote path
    value      Value       // Cached value
    timestamp  time.Time   // Cache time
    version    uint64      // Data version
    dirty      bool        // Needs write-back
}
```

#### Cache Consistency Strategy

1. **Read-Through Caching:**
   - Cache miss triggers remote fetch
   - Cached data has configurable TTL
   - Version tracking for consistency

2. **Write-Through Caching:**
   - Writes go directly to remote mesh
   - Local cache updated on successful write
   - Dirty flag for failed writes

3. **Cache Invalidation:**
   - Time-based expiration (TTL)
   - Manual invalidation on errors
   - Heartbeat-based freshness checks

### 6. Network Protocol Extensions

#### Bridge-Specific Messages

```go
type MessageType int
const (
    // Existing messages...
    CREATE_BRIDGE MessageType = iota + 100
    BRIDGE_AUTH
    BRIDGE_CHALLENGE
    BRIDGE_RESPONSE
    BRIDGE_DATA_READ
    BRIDGE_DATA_WRITE
    BRIDGE_DISCONNECT
)

type CreateBridgeMessage struct {
    OriginMesh    string `json:"origin_mesh"`
    RequestedID   string `json:"requested_id"`
    PublicKey     []byte `json:"public_key"`
}

type BridgeDataMessage struct {
    BridgeID      string   `json:"bridge_id"`
    Path          []string `json:"path"`
    Value         Value    `json:"value,omitempty"`
    Operation     string   `json:"operation"` // "read" or "write"
}
```

#### Protocol Flow

1. **Bridge Establishment:**
   ```
   Node A -> Node B: CREATE_BRIDGE {origin_mesh, identity, public_key}
   Node B -> Node A: CREATE_BRIDGE_ACK {assigned_identity, session_id}
   ```

2. **Authentication:**
   ```
   Node A -> Node B: BRIDGE_AUTH {session_id, identity}
   Node B -> Node A: BRIDGE_CHALLENGE {challenge, timestamp}
   Node A -> Node B: BRIDGE_RESPONSE {signature}
   Node B -> Node A: BRIDGE_AUTH_ACK {status, session_token}
   ```

3. **Data Operations:**
   ```
   Node A -> Node B: BRIDGE_DATA_READ {bridge_id, path}
   Node B -> Node A: BRIDGE_DATA_RESPONSE {value, version, timestamp}
   ```

### 7. Error Handling and Recovery

#### Bridge Failure Scenarios

1. **Connection Loss:**
   - Automatic reconnection with exponential backoff
   - Session resume using stored tokens
   - Cache persistence during disconnection

2. **Authentication Failure:**
   - Key re-derivation and retry
   - Identity validation with mesh
   - Fallback to discovery protocol

3. **Data Consistency Errors:**
   - Version conflict resolution
   - Cache invalidation and refresh
   - Rollback on write failures

#### Recovery Strategies

```go
type BridgeRecovery struct {
    maxRetries    int
    retryDelay    time.Duration
    backoffFactor float64
    healthCheck   func() bool
}

func (br *BridgeRecovery) HandleConnectionLoss(bridge *BridgeConnection) error {
    for attempt := 0; attempt < br.maxRetries; attempt++ {
        // Wait with exponential backoff
        delay := time.Duration(float64(br.retryDelay) * math.Pow(br.backoffFactor, float64(attempt)))
        time.Sleep(delay)

        // Attempt reconnection
        if err := bridge.Reconnect(); err == nil {
            // Re-authenticate
            if err := bridge.ReAuthenticate(); err == nil {
                return nil // Success
            }
        }
    }

    // Mark bridge as failed after max retries
    bridge.Status = ConnectionFailed
    return ErrMaxRetriesExceeded
}
```

## Performance Characteristics

### Latency Analysis

- **Bridge Creation:** ~100-500ms (includes discovery + authentication)
- **Data Access:** ~10-50ms per operation (cached: ~1ms)
- **Authentication:** ~50-100ms per challenge-response cycle

### Memory Usage

- **Per Bridge:** ~1-5MB (connection state + cache)
- **Cache Entry:** ~100-500 bytes per cached item
- **Identity Storage:** ~50-100 bytes per identity

### Network Overhead

- **Bridge Maintenance:** ~1KB/sec per active bridge (heartbeats)
- **Data Operations:** ~10% protocol overhead over raw data
- **Authentication:** ~2KB per auth cycle

## Security Considerations

### Cryptographic Properties

1. **Key Derivation:** PBKDF2 with 100,000 iterations
2. **Signatures:** Ed25519 elliptic curve signatures
3. **Random Generation:** Cryptographically secure random numbers
4. **Salt Usage:** Mesh-specific salts prevent rainbow table attacks

### Access Control

1. **Identity Validation:** All operations require valid bridge identity
2. **Path Restrictions:** Bridge agents limited to their agent home
3. **Session Management:** Time-limited authentication sessions
4. **Audit Trail:** All bridge operations logged for security review

### Threat Mitigation

1. **Man-in-the-Middle:** TLS encryption for all network traffic
2. **Replay Attacks:** Timestamp validation and nonce usage
3. **Identity Theft:** Private key protection and device binding
4. **Data Tampering:** Cryptographic signatures on all operations

## Future Enhancements

### Planned Features

1. **Bridge Load Balancing:** Multiple bridge connections per mesh
2. **Hierarchical Bridging:** Bridge chains across multiple meshes
3. **Data Synchronization:** Automatic data sync between meshes
4. **Policy Management:** Fine-grained access control policies

### Performance Optimizations

1. **Connection Pooling:** Shared connections for multiple bridge identities
2. **Batch Operations:** Bulk data operations across bridges
3. **Predictive Caching:** ML-based cache pre-loading
4. **Compression:** Data compression for network efficiency

## Implementation Guidelines

### For Bridge Clients

1. **Connection Management:** Use connection pooling for efficiency
2. **Error Handling:** Implement robust retry logic with backoff
3. **Cache Strategy:** Balance cache size with access patterns
4. **Security:** Protect private keys and session tokens

### For Mesh Operators

1. **Network Planning:** Consider bridge latency in mesh design
2. **Access Control:** Carefully control which meshes can bridge
3. **Monitoring:** Track bridge performance and security events
4. **Capacity Planning:** Account for bridge overhead in resource planning

---

*Related Documentation:*
- [Mesh Management Guide](mesh_management_guide.md) - User-facing operations
- [AmorphDB Design](amorphdb_design.md) - Core system architecture
- [Security Framework](../internal/security/README.md) - Security implementation details