# AmorphDB Mesh Bridge Implementation Plan

**Status:** In Progress
**Started:** 2026-03-14
**Target:** Full mesh bridging and mobile agent implementation

## Overview

This plan implements the updated AmorphDB mesh architecture with:
- Explicit mesh creation and naming
- Bridge connections between multiple meshes
- Mobile agent vs node agent distinction
- Multi-mesh identity management
- Discovery-based mesh joining

## Current State Assessment

### ✅ Working Systems
- [x] Core storage engine with temporal tree-graph persistence
- [x] Complete MBL language implementation (lexer, parser, interpreter)
- [x] Reactive programming with procedures and watchers
- [x] Object model with heritability and instantiation
- [x] Security framework (permissions, stamps, filters)
- [x] Basic service architecture (daemon, client, protocol)
- [x] Distributed mesh networking with zone splitting
- [x] Basic authentication and encryption

### ⚠️ Needs Implementation
- [ ] Explicit mesh creation commands
- [ ] Bridge connection management
- [ ] Mobile agent identity storage in mesh
- [ ] Multi-mesh data structures
- [ ] Discovery protocol for mesh joining
- [ ] Bridge authentication across meshes

## Implementation Plan

### Phase 1: Foundation Changes ✅ Ready to Start

#### Step 1.1: Configuration System Updates
**Objective:** Update config system to support mesh names and multi-mesh storage

**Files to Modify:**
- `internal/config/config.go` - Add mesh configuration structures
- `cmd/amorphd/main.go` - Add mesh name command line flag
- `cmd/amorphctl/main.go` - Add mesh management commands

**Implementation:**
```go
type MeshConfig struct {
    Name     string            `yaml:"name"`      // Mesh name (empty = standalone)
    Bridges  map[string]string `yaml:"bridges"`   // Bridge connections
    Identity string            `yaml:"identity"`  // Node identity
}

type Config struct {
    Mesh     MeshConfig        `yaml:"mesh"`
    Data     DataConfig        `yaml:"data"`
    Network  NetworkConfig     `yaml:"network"`
    // existing fields...
}
```

**Testing:**
- [ ] Config parsing with mesh sections
- [ ] Default standalone configuration
- [ ] Invalid mesh name rejection
- [ ] Multi-mesh bridge configuration

**Completion Criteria:**
- [x] Config structures defined
- [ ] Config file parsing working
- [ ] Command line flag integration
- [ ] Test coverage ≥90%

---

#### Step 1.2: Node vs Mobile Agent Identity System
**Objective:** Implement separate identity management for node and mobile agents

**Files to Modify:**
- `internal/identity/identity.go` - Create identity management system
- `internal/storage/agent.go` - Add agent home storage distinction
- `internal/types/agent.go` - Define agent type distinctions

**Implementation:**
```go
type AgentType int
const (
    NodeAgent   AgentType = iota  // Local storage only
    MobileAgent                   // Mesh storage, replicated
)

type Identity struct {
    ID       string    `json:"id"`
    Type     AgentType `json:"type"`
    MeshName string    `json:"mesh_name"`  // Which mesh this identity belongs to
}

type IdentityManager struct {
    nodeIdentity   Identity              // This node's identity
    bridgeIdentities map[string]Identity // Bridge identities by mesh name
}
```

**Testing:**
- [ ] Node identity generation and storage
- [ ] Mobile agent identity assignment
- [ ] Bridge identity management
- [ ] Identity conflict detection

**Completion Criteria:**
- [ ] Identity structures implemented
- [ ] Local node identity storage
- [ ] Mobile agent identity in mesh storage
- [ ] Bridge identity tracking
- [ ] Test coverage ≥90%

---

### Phase 2: Mesh Management Commands

#### Step 2.1: Create-Mesh Command
**Objective:** Implement `amorphctl create-mesh <name>` functionality

**Files to Modify:**
- `cmd/amorphctl/mesh.go` - Add mesh creation command
- `internal/mesh/manager.go` - Add mesh initialization logic
- `internal/service/mesh.go` - Service integration

**Implementation:**
```go
func (m *MeshManager) CreateMesh(name string) error {
    if err := validateMeshName(name); err != nil {
        return err
    }

    // Update node configuration
    m.config.Mesh.Name = name
    m.nodeIdentity = generateNodeIdentity()

    // Initialize mesh state
    return m.initializeMeshFounder()
}

func validateMeshName(name string) error {
    // Alphanumeric + hyphens/underscores only
    // No spaces, @ symbols, or special characters
}
```

**Testing:**
- [ ] Valid mesh name creation
- [ ] Invalid mesh name rejection
- [ ] Node becomes mesh founder
- [ ] Mesh state persistence

**Completion Criteria:**
- [ ] `create-mesh` command functional
- [ ] Mesh name validation working
- [ ] Node identity generation
- [ ] Configuration persistence
- [ ] Test coverage ≥90%

---

#### Step 2.2: Join Command with Discovery
**Objective:** Implement `amorphctl join <address>` with mesh name discovery

**Files to Modify:**
- `cmd/amorphctl/join.go` - Add join command
- `internal/mesh/discovery.go` - Add mesh discovery protocol
- `internal/network/handshake.go` - Update handshake for discovery

**Implementation:**
```go
func (m *MeshManager) JoinMesh(seedAddress string) error {
    // Connect and discover mesh name
    conn, err := m.connectToSeed(seedAddress)
    if err != nil {
        return err
    }

    meshInfo, err := m.discoverMeshInfo(conn)
    if err != nil {
        return err
    }

    // Preserve existing world data if present
    if err := m.preserveExistingData(meshInfo.Name); err != nil {
        return err
    }

    return m.joinDiscoveredMesh(meshInfo)
}
```

**Testing:**
- [ ] Successful mesh discovery
- [ ] Handshake protocol integration
- [ ] Existing data preservation
- [ ] Zone assignment on join

**Completion Criteria:**
- [ ] `join` command functional
- [ ] Mesh name discovery working
- [ ] Data preservation on join
- [ ] Zone redistribution
- [ ] Test coverage ≥90%

---

#### Step 2.3: Bridge Command
**Objective:** Implement `amorphctl bridge <address>` for multi-mesh connections

**Files to Modify:**
- `cmd/amorphctl/bridge.go` - Add bridge command
- `internal/mesh/bridge.go` - Bridge management logic
- `internal/identity/bridge.go` - Bridge identity management

**Implementation:**
```go
type BridgeManager struct {
    primaryMesh  string
    bridges      map[string]*BridgeConnection
    identities   map[string]Identity  // mesh_name -> identity
}

func (m *MeshManager) CreateBridge(targetAddress string) error {
    // Discover target mesh
    meshInfo, err := m.discoverMeshInfo(targetAddress)
    if err != nil {
        return err
    }

    // Request identity assignment in target mesh
    bridgeIdentity, err := m.requestBridgeIdentity(meshInfo)
    if err != nil {
        return err
    }

    // Establish bridge connection
    return m.establishBridge(meshInfo, bridgeIdentity)
}
```

**Testing:**
- [ ] Bridge connection establishment
- [ ] Multi-mesh identity assignment
- [ ] Bridge data access patterns
- [ ] Bridge disconnection

**Completion Criteria:**
- [ ] `bridge` command functional
- [ ] Multiple mesh connections
- [ ] Bridge identity tracking
- [ ] Cross-mesh data access
- [ ] Test coverage ≥90%

---

#### Step 2.4: Detach Command
**Objective:** Implement `amorphctl detach [mesh-name]` for graceful disconnection

**Files to Modify:**
- `cmd/amorphctl/detach.go` - Add detach command
- `internal/mesh/leave.go` - Graceful departure logic

**Implementation:**
```go
func (m *MeshManager) DetachFromMesh(meshName string) error {
    if meshName == "" {
        // Detach from primary mesh
        return m.leavePrimaryMesh()
    }

    // Detach from specific bridge
    return m.disconnectBridge(meshName)
}

func (m *MeshManager) leavePrimaryMesh() error {
    // Announce departure via gossip
    // Migrate owned zones to adjacent nodes
    // Clear mesh state, become standalone
}
```

**Testing:**
- [ ] Primary mesh departure
- [ ] Bridge disconnection
- [ ] Zone migration on departure
- [ ] Clean state transitions

**Completion Criteria:**
- [ ] `detach` command functional
- [ ] Graceful mesh departure
- [ ] Zone migration working
- [ ] State cleanup complete
- [ ] Test coverage ≥90%

---

### Phase 3: Data Structure Updates

#### Step 3.1: Multi-Mesh Data Access
**Objective:** Implement `my.partnernet.*` data access patterns

**Files to Modify:**
- `internal/storage/bridge.go` - Bridge data storage
- `internal/interpreter/bridge_scope.go` - Bridge scope resolution
- `internal/types/path.go` - Multi-mesh path handling

**Implementation:**
```go
type BridgeScope struct {
    meshName     string
    agentHome    string           // world.agent.{bridge_identity}
    localPath    string           // my.partnernet.*
}

func (i *Interpreter) resolveBridgePath(path string) (*BridgeScope, error) {
    if !strings.HasPrefix(path, "my.") {
        return nil, nil  // Not a bridge path
    }

    parts := strings.Split(path, ".")
    if len(parts) < 2 {
        return nil, nil
    }

    meshName := parts[1]
    if bridge := i.bridges[meshName]; bridge != nil {
        return &BridgeScope{
            meshName:  meshName,
            agentHome: fmt.Sprintf("world.agent.%s", bridge.Identity.ID),
            localPath: path,
        }, nil
    }

    return nil, nil  // Not a bridge mesh
}
```

**Testing:**
- [ ] Bridge path resolution
- [ ] Cross-mesh data queries
- [ ] Bridge data modifications
- [ ] Path conflict handling

**Completion Criteria:**
- [ ] `my.meshname.*` paths working
- [ ] Bridge data access functional
- [ ] Cross-mesh operations
- [ ] Error handling complete
- [ ] Test coverage ≥90%

---

#### Step 3.2: Bridge Authentication Integration
**Objective:** Implement mobile agent authentication for bridges

**Files to Modify:**
- `internal/auth/bridge.go` - Bridge authentication
- `internal/crypto/mobile.go` - Mobile agent key derivation
- `internal/mesh/agent_handshake.go` - Agent handshake protocol

**Implementation:**
```go
type BridgeAuth struct {
    meshIdentities map[string]*MobileIdentity  // mesh -> identity
    derivedKeys    map[string]*DerivedKey      // mesh -> keys
}

func (b *BridgeAuth) AuthenticateToMesh(meshName string, connection *Connection) error {
    identity := b.meshIdentities[meshName]
    if identity == nil {
        return fmt.Errorf("no identity for mesh %s", meshName)
    }

    // Derive authentication key for this mesh
    key, err := b.deriveKeyForMesh(meshName, identity)
    if err != nil {
        return err
    }

    // Perform challenge-response authentication
    return b.performChallengeResponse(connection, identity, key)
}
```

**Testing:**
- [ ] Bridge authentication handshake
- [ ] Mobile agent key derivation
- [ ] Multi-mesh key management
- [ ] Authentication persistence

**Completion Criteria:**
- [ ] Bridge authentication working
- [ ] Mobile agent patterns
- [ ] Key derivation functional
- [ ] Multi-mesh auth complete
- [ ] Test coverage ≥90%

---

### Phase 4: Integration and Testing

#### Step 4.1: End-to-End Integration
**Objective:** Complete integration testing of all mesh bridge features

**Test Scenarios:**
- [ ] **Scenario A:** Standalone → Create Mesh → Add Members
  - Start standalone node
  - Create named mesh
  - Join second node
  - Verify mesh functionality

- [ ] **Scenario B:** Multi-Mesh Bridge Operations
  - Create two separate meshes
  - Bridge node connects to both
  - Cross-mesh data operations
  - Bridge disconnection

- [ ] **Scenario C:** Data Preservation and Migration
  - Standalone node with data
  - Join mesh (preserves data in `my.standalone.*`)
  - Manual data migration
  - Mesh departure

- [ ] **Scenario D:** Mobile Agent Bridge Access
  - Bridge node acting as mobile agent
  - Identity assignment in partner mesh
  - Agent home replication
  - Cross-mesh agent operations

**Files to Create:**
- `tests/integration/mesh_lifecycle_test.go`
- `tests/integration/bridge_operations_test.go`
- `tests/integration/data_migration_test.go`
- `tests/integration/mobile_agent_bridge_test.go`

**Completion Criteria:**
- [ ] All integration scenarios pass
- [ ] Performance benchmarks meet targets
- [ ] Memory usage within limits
- [ ] Network protocol efficiency verified
- [ ] Error recovery tested

---

#### Step 4.2: Documentation and Examples
**Objective:** Complete user-facing documentation and examples

**Files to Create/Update:**
- `docs/mesh_management_guide.md` - User guide for mesh operations
- `docs/bridge_architecture.md` - Technical bridge architecture
- `examples/mesh_setup/` - Example deployment scripts
- `examples/bridge_workflows/` - Bridge usage examples

**Content:**
- [ ] Mesh creation and joining workflows
- [ ] Bridge setup and management
- [ ] Mobile agent authentication
- [ ] Cross-mesh data patterns
- [ ] Troubleshooting guide

**Completion Criteria:**
- [ ] User guide complete and tested
- [ ] Examples working and verified
- [ ] Architecture documentation updated
- [ ] API reference updated

---

## Implementation Priority

### Critical Path (Week 1-2)
1. **Step 1.1** - Configuration System Updates
2. **Step 1.2** - Identity System Foundation
3. **Step 2.1** - Create-Mesh Command

### Core Features (Week 3-4)
4. **Step 2.2** - Join Command with Discovery
5. **Step 2.3** - Bridge Command
6. **Step 3.1** - Multi-Mesh Data Access

### Integration (Week 5-6)
7. **Step 2.4** - Detach Command
8. **Step 3.2** - Bridge Authentication
9. **Step 4.1** - End-to-End Integration

### Polish (Week 7)
10. **Step 4.2** - Documentation and Examples

## Risk Mitigation

### Technical Risks
- **Identity Conflicts:** Mitigated by mesh-specific identity assignment
- **Bridge Authentication:** Mitigated by mobile agent pattern
- **Data Consistency:** Mitigated by existing zone replication
- **Performance Impact:** Mitigated by efficient bridge connection management

### Implementation Risks
- **Complexity:** Broken into small, testable steps
- **Backward Compatibility:** All changes are additive to existing functionality
- **Testing Coverage:** Each step requires ≥90% test coverage
- **Documentation:** Parallel documentation development

## Success Criteria

### Functional Requirements
- [x] Standalone nodes work as before
- [ ] Explicit mesh creation and naming
- [ ] Discovery-based mesh joining
- [ ] Multi-mesh bridge connections
- [ ] Mobile agent identity management
- [ ] Cross-mesh data operations
- [ ] Graceful mesh departure

### Performance Requirements
- [ ] Bridge operations ≤10ms latency overhead
- [ ] Memory usage ≤5MB per bridge connection
- [ ] Network bandwidth ≤5% increase per bridge
- [ ] Zone migration completes ≤30 seconds

### Quality Requirements
- [ ] Test coverage ≥90% for all new code
- [ ] Zero regression in existing functionality
- [ ] Complete documentation coverage
- [ ] Production-ready error handling

## Progress Tracking

**Current Step:** Ready to begin Step 1.1
**Overall Progress:** 0% (0/10 steps completed)
**Estimated Completion:** 2026-04-25

### Weekly Checkpoints
- **Week 1:** Steps 1.1-1.2 complete
- **Week 2:** Steps 2.1-2.2 complete
- **Week 3:** Steps 2.3-3.1 complete
- **Week 4:** Steps 2.4-3.2 complete
- **Week 5:** Step 4.1 complete
- **Week 6:** Integration testing complete
- **Week 7:** Documentation and polish complete

### Session Recovery Notes
*This section will be updated with key context for session recovery*

- Design document updated with mesh bridging concepts
- Current codebase is functional but needs mesh management commands
- Priority is configuration system foundation before building commands
- Testing infrastructure exists and should be used throughout
- Memory management lessons learned from VM testing (limit concurrent operations)

---

**Last Updated:** 2026-03-14
**Next Review:** 2026-03-21