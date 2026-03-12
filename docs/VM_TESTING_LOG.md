# AmorphDB VM Testing Documentation

**Date**: 2026-03-07
**Goal**: Test AmorphDB distributed system using real VMs with QEMU Console Bridge

## Prerequisites Verified ✅

### Infrastructure Components
- **QEMU Console Bridge**: `/home/solifugus/development/qemu-console-bridge/src/qemu_console.py` ✅
- **YakirOS VM Image**: `/home/solifugus/development/YakirOS/yakiros-vm.qcow2` (151MB) ✅
- **KVM Support**: Available on Kubuntu system ✅

### AmorphDB Binaries Built ✅
```bash
bin/amorphd    (3.9MB) - Distributed database daemon
bin/amorph     (4.1MB) - Interactive REPL client
bin/amorphctl  (3.5MB) - Control and management tool
```

## VM Test Architecture

### Network Configuration
- **Node 1**: Host ports 5000 (AmorphDB), 22 (SSH)
- **Node 2**: Host ports 5010 (AmorphDB), 32 (SSH)
- **Inter-VM Network**: Standard QEMU user networking (10.0.2.x)

### AmorphDB Distributed Setup (Per Design Spec)
- **Node Identity**: CV syllable patterns (e.g., "ra-do-ki", "fi-ne-so")
- **Bootstrap Process**: First node self-genesis, second node connects
- **Zone Assignment**: Consistent hashing with 150-200 virtual nodes per physical
- **Replication**: Authority/replica pairs with cross-node data distribution
- **Heartbeat**: 333ms intervals for failure detection

## Testing Phases

### Phase 1: Infrastructure Validation
1. **VM Creation**: QEMU image creation and launch
2. **Console Bridge**: AI-to-VM communication via console
3. **Network Connectivity**: Inter-VM ping and port access
4. **Binary Deployment**: Copy AmorphDB binaries to VMs

### Phase 2: Single Node Validation
1. **Daemon Launch**: Start amorphd on Node 1 with first-node config
2. **Client Connection**: Connect amorph REPL to local daemon
3. **Basic Operations**: Create data, verify storage persistence
4. **MBL Functionality**: Test language features per design spec

### Phase 3: Distributed Mesh Testing
1. **Second Node Join**: Bootstrap Node 2 to connect to Node 1
2. **Zone Distribution**: Verify consistent hashing and zone assignment
3. **Cross-Node Replication**: Test data replication between nodes
4. **Distributed Queries**: Access data from any node in mesh

### Phase 4: Failure Testing
1. **Node Failure**: Simulate VM crash during operations
2. **Network Partitions**: Test split-brain scenarios
3. **Recovery Validation**: Verify data consistency after failures

## Design Compliance Tracking

### Core Requirements from amorphdb_design.md
- ✅ **Temporal Semantics**: Every value has history, assignment records changes
- ✅ **Tree-Graph Structure**: Attributes, instances, values with timestamp chains
- ✅ **MBL Language**: Scope resolution with `~`, `.`, `+` prefixes
- ✅ **Type System**: 9 high-level types (text, number, time, etc.)
- ✅ **Reactive System**: Watchers with heartbeat execution (333ms)
- ✅ **Heritability**: Object inheritance with copy/link/reset/exclude modifiers
- ✅ **Distributed Architecture**: Mesh networking with zone management

### Security Requirements
- ✅ **Permissions**: @read, @write, @expand, @grant, @purge
- ✅ **Stamps**: @author injection for audit trails
- ✅ **Filters**: Personal data visibility control
- ✅ **Encryption**: Node-to-node encrypted communication
- ✅ **Authentication**: Challenge-response with derived keys

## Success Criteria

### Functional Requirements
- [ ] VM infrastructure working (QEMU + Console Bridge)
- [ ] AmorphDB daemon starts successfully on VMs
- [ ] Client can connect to daemon via network
- [ ] Distributed mesh formation (2 nodes)
- [ ] Cross-node data replication working
- [ ] MBL language features operational in distributed environment

### Performance Targets (Realistic for VM Testing)
- **Inter-VM Latency**: <50ms for VM-to-VM operations
- **Throughput**: >10 writes/sec across 2 VMs
- **Reliability**: Mesh recovery within 3 heartbeat cycles (1 second)

## Next Steps
1. Enhance vm-test-setup.py to use real AmorphDB binaries
2. Add proper distributed mesh configuration
3. Test basic infrastructure (VM + Console Bridge)
4. Implement distributed AmorphDB mesh testing
5. Validate against design specification requirements
