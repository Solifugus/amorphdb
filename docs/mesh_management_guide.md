# AmorphDB Mesh Management Guide

## Overview

AmorphDB supports distributed mesh networking, allowing multiple nodes to form named meshes and bridge connections across different meshes. This guide covers all mesh operations from basic setup to advanced multi-mesh bridging.

## Quick Start

### Starting as Standalone Node

By default, AmorphDB nodes start in standalone mode:

```bash
# Start a standalone node
amorphd --port 8080

# Verify standalone status
amorphctl status
```

**Expected Output:**
```
Status: Standalone
Mesh: (none)
Identity: standalone-node-001
Storage: /var/lib/amorphdb
```

### Creating Your First Mesh

Convert a standalone node into a mesh founder:

```bash
# Create a new mesh named "my-company"
amorphctl create-mesh my-company

# Verify mesh creation
amorphctl status
```

**Expected Output:**
```
Status: Mesh Founder
Mesh: my-company
Identity: node-a1b2c3d4
Storage: /var/lib/amorphdb
```

**Important:** The node that creates a mesh becomes its founder. All data previously stored in the standalone node remains accessible.

## Mesh Operations

### Joining an Existing Mesh

To join an existing mesh, you need the address of any current mesh member:

```bash
# Join a mesh by connecting to a known member
amorphctl join 192.168.1.100:8080

# The mesh name is automatically discovered
# Your node will be assigned to a zone automatically
```

**What Happens During Join:**
1. **Discovery:** Connect to the provided address and discover the mesh name
2. **Identity Assignment:** Receive a new node identity within the mesh
3. **Data Preservation:** Existing data is preserved under `my.standalone.*`
4. **Zone Assignment:** Automatically assigned to a zone for load balancing

### Checking Mesh Status

```bash
# Get detailed mesh information
amorphctl status --verbose

# List all mesh members
amorphctl mesh members

# Show zone distribution
amorphctl mesh zones
```

**Sample Verbose Output:**
```
Status: Mesh Member
Mesh: my-company
Identity: node-e5f6g7h8
Zone: zone-2 (responsible for paths: k-p)
Members: 3 active nodes
Bridges: 1 connection (partner-network)
Storage: /var/lib/amorphdb
Uptime: 2h 34m
```

### Leaving a Mesh

To gracefully leave a mesh and return to standalone mode:

```bash
# Leave the current mesh
amorphctl detach

# Verify standalone status
amorphctl status
```

**What Happens During Detach:**
1. **Zone Migration:** Your zones are redistributed to other nodes
2. **Data Preservation:** Mesh data is preserved locally
3. **Clean Departure:** Other nodes are notified of your departure
4. **Standalone Mode:** Node returns to standalone operation

## Bridge Connections

### Understanding Bridges

Bridges allow a single node to participate in multiple meshes simultaneously. The node acts as a "mobile agent" in partner meshes while maintaining its primary mesh membership.

### Creating a Bridge

```bash
# Connect to another mesh as a bridge
amorphctl bridge 192.168.2.100:8080

# This discovers the target mesh and establishes a bridge connection
```

**What Happens During Bridge Creation:**
1. **Mesh Discovery:** Target mesh name is discovered automatically
2. **Identity Assignment:** You receive a mobile agent identity in the partner mesh
3. **Authentication Setup:** Cryptographic keys are derived for secure access
4. **Data Access:** You can now access data in both meshes

### Working with Bridge Data

Once a bridge is established, you can access data in the partner mesh using the `my.meshname.*` pattern:

```bash
# Access data in the partner mesh (assuming it's named "partner-network")
amorphctl get my.partner-network.shared.config

# Write data to the partner mesh
amorphctl set my.partner-network.projects.demo "Bridge test data"

# Your data appears in the partner mesh under world.agent.{your-bridge-id}
```

**Data Mapping:**
- `my.partner-network.projects.demo` in your mesh
- `world.agent.a1b2c3d4.projects.demo` in partner mesh (where a1b2c3d4 is your bridge identity)

### Managing Multiple Bridges

```bash
# List all active bridges
amorphctl bridge list

# Check bridge status
amorphctl bridge status partner-network

# Disconnect from a specific mesh
amorphctl detach partner-network
```

## Data Management Patterns

### Data Organization

Understanding how data is organized in mesh environments:

```
/world/
├── shared/          # Mesh-wide shared data
│   ├── config/      # Mesh configuration
│   └── templates/   # Shared templates
├── agent/           # Agent-specific data
│   ├── {node-id}/   # Local node data
│   └── {bridge-id}/ # Bridge agent data
└── zones/           # Zone-specific data
    ├── zone-1/      # Zone 1 data
    └── zone-2/      # Zone 2 data

/my/
├── standalone/      # Preserved standalone data
└── {mesh-name}/     # Bridge access to other meshes
```

### Data Preservation Examples

**Before Joining a Mesh:**
```
/local/users/admin/profile = "Local admin"
```

**After Joining a Mesh:**
```
/my/standalone/local/users/admin/profile = "Local admin"  # Preserved
/world/shared/config/mesh_name = "my-company"             # New mesh data
```

**After Creating a Bridge:**
```
/my/standalone/local/users/admin/profile = "Local admin"      # Preserved
/my/partner-network/shared/config/version = "2.1.0"          # Bridge access
/world/agent/{bridge-id}/local/cache = "Bridge data"         # Your bridge data
```

## Advanced Scenarios

### Multi-Mesh Data Synchronization

Example workflow for keeping data synchronized across meshes:

```bash
# Set up bridge to partner mesh
amorphctl bridge partner-mesh.example.com:8080

# Read configuration from partner mesh
CONFIG=$(amorphctl get my.partner-mesh.shared.config.version)

# Update local mesh with partner configuration
amorphctl set world.shared.partner_config.version "$CONFIG"

# Write status back to partner mesh
amorphctl set my.partner-mesh.sync.status "Updated from my-company mesh"
```

### Mesh Migration Strategy

Moving data between meshes safely:

```bash
# 1. Join target mesh first
amorphctl join target-mesh.example.com:8080

# 2. Create bridge back to original mesh (requires another node)
amorphctl bridge original-mesh.example.com:8080

# 3. Copy critical data
amorphctl get my.original-mesh.critical.data > backup.json
amorphctl set world.shared.migrated.data "$(cat backup.json)"

# 4. Verify data integrity
amorphctl get world.shared.migrated.data

# 5. Disconnect from original mesh
amorphctl detach original-mesh
```

## Troubleshooting

### Common Issues

**Problem:** Cannot join mesh - "Connection refused"
```bash
# Check if the target node is reachable
ping 192.168.1.100

# Check if the target port is open
telnet 192.168.1.100 8080

# Verify your node isn't already in a mesh
amorphctl status
```

**Problem:** Bridge authentication fails
```bash
# Check bridge status
amorphctl bridge status target-mesh

# Verify network connectivity
amorphctl bridge ping target-mesh

# Reset bridge authentication (caution: may lose access)
amorphctl bridge reset target-mesh
```

**Problem:** Data not appearing in partner mesh
```bash
# Verify bridge is active
amorphctl bridge list

# Check your bridge identity
amorphctl bridge identity target-mesh

# Test access pattern
amorphctl set my.target-mesh.test.probe "$(date)"
amorphctl get my.target-mesh.test.probe
```

### Diagnostic Commands

```bash
# Full system status
amorphctl status --verbose

# Network connectivity test
amorphctl mesh ping

# Bridge connectivity test
amorphctl bridge ping --all

# Storage integrity check
amorphctl validate storage

# Zone distribution analysis
amorphctl mesh zones --detailed
```

### Log Analysis

Check logs for detailed error information:

```bash
# View recent mesh operations
journalctl -u amorphd -f --grep "MESH"

# Check bridge-specific logs
journalctl -u amorphd -f --grep "BRIDGE"

# Authentication troubleshooting
journalctl -u amorphd -f --grep "AUTH"
```

## Best Practices

### Mesh Design

1. **Naming Convention:** Use descriptive, hierarchical mesh names
   - Good: `company-engineering`, `project-alpha-test`
   - Avoid: `mesh1`, `temp`, special characters

2. **Zone Planning:** Consider geographic and network topology
   - Place nodes in same datacenter in same zone when possible
   - Minimize cross-zone traffic for performance

3. **Bridge Strategy:** Limit bridge connections to avoid complexity
   - Use dedicated bridge nodes for high-traffic connections
   - Monitor bridge latency and disconnect unused bridges

### Security Considerations

1. **Network Security:** Use TLS and proper firewall rules
2. **Bridge Access:** Carefully control which meshes can bridge to yours
3. **Data Isolation:** Use proper access patterns to isolate sensitive data
4. **Identity Management:** Regularly rotate mobile agent keys

### Performance Optimization

1. **Zone Distribution:** Balance load across zones
2. **Bridge Efficiency:** Minimize unnecessary cross-mesh operations
3. **Data Locality:** Keep frequently accessed data in same zone
4. **Connection Pooling:** Reuse bridge connections when possible

## API Reference Summary

### Core Commands

| Command | Description | Example |
|---------|-------------|---------|
| `amorphctl create-mesh <name>` | Create new mesh | `amorphctl create-mesh my-company` |
| `amorphctl join <address>` | Join existing mesh | `amorphctl join 192.168.1.100:8080` |
| `amorphctl bridge <address>` | Create bridge connection | `amorphctl bridge partner.example.com:8080` |
| `amorphctl detach [mesh-name]` | Leave mesh or disconnect bridge | `amorphctl detach partner-mesh` |
| `amorphctl status` | Show current status | `amorphctl status --verbose` |

### Mesh Information

| Command | Description | Example |
|---------|-------------|---------|
| `amorphctl mesh members` | List mesh members | `amorphctl mesh members` |
| `amorphctl mesh zones` | Show zone distribution | `amorphctl mesh zones --detailed` |
| `amorphctl mesh ping` | Test mesh connectivity | `amorphctl mesh ping` |

### Bridge Operations

| Command | Description | Example |
|---------|-------------|---------|
| `amorphctl bridge list` | List active bridges | `amorphctl bridge list` |
| `amorphctl bridge status <mesh>` | Check bridge status | `amorphctl bridge status partner-mesh` |
| `amorphctl bridge identity <mesh>` | Show bridge identity | `amorphctl bridge identity partner-mesh` |
| `amorphctl bridge ping <mesh>` | Test bridge connectivity | `amorphctl bridge ping partner-mesh` |

## Configuration Reference

### Mesh Configuration Block

```yaml
mesh:
  name: "my-company"           # Mesh name (empty for standalone)
  identity: "node-001"         # Node identity string
  bridges:                     # Bridge connections
    partner-mesh: "192.168.2.100:8080"
    test-mesh: "10.0.1.50:8080"
```

### Network Configuration

```yaml
network:
  port: 8080                   # Main service port
  local_socket_path: "/var/run/amorphdb.sock"  # Local IPC socket
  max_connections: 100         # Connection limit
  bridge_timeout: 30           # Bridge timeout in seconds
```

---

*For technical implementation details, see [Bridge Architecture Guide](bridge_architecture.md)*
*For deployment examples, see the [examples/](../examples/) directory*