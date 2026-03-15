#!/bin/bash
# AmorphDB Three-Node Local Mesh Setup
#
# Creates a three-node mesh running on localhost for testing
# distributed features like zone splitting and replication.

set -euo pipefail

# Configuration
MESH_NAME="${MESH_NAME:-test-mesh-local}"
BASE_DIR="${BASE_DIR:-$HOME/.amorphdb-mesh}"
BASE_PORT="${BASE_PORT:-8080}"

echo "🚀 Setting up three-node AmorphDB mesh..."
echo "   Mesh name: $MESH_NAME"
echo "   Base directory: $BASE_DIR"
echo "   Ports: $BASE_PORT, $((BASE_PORT+1)), $((BASE_PORT+2))"

# Clean up previous setup
if [ -d "$BASE_DIR" ]; then
    echo "🧹 Cleaning up previous setup..."
    pkill -f "amorphd.*$BASE_DIR" || true
    sleep 2
    rm -rf "$BASE_DIR"
fi

# Create directory structure
for i in 1 2 3; do
    mkdir -p "$BASE_DIR/node$i/data"
    mkdir -p "$BASE_DIR/node$i/logs"
done

# Node 1 - Mesh founder
cat > "$BASE_DIR/node1/config.yaml" << EOF
data:
  storage_dir: "$BASE_DIR/node1/data"

network:
  port: $BASE_PORT
  local_socket_path: "$BASE_DIR/node1/amorphdb.sock"
  max_connections: 100

mesh:
  name: ""  # Will be set when creating mesh
  identity: "node1-founder"

logging:
  level: "info"
  file: "$BASE_DIR/node1/logs/amorphdb.log"

security:
  enable_auth: false  # Simplified for local testing
EOF

# Node 2 - Mesh member
cat > "$BASE_DIR/node2/config.yaml" << EOF
data:
  storage_dir: "$BASE_DIR/node2/data"

network:
  port: $((BASE_PORT+1))
  local_socket_path: "$BASE_DIR/node2/amorphdb.sock"
  max_connections: 100

mesh:
  name: ""  # Will be set when joining mesh
  identity: "node2-member"

logging:
  level: "info"
  file: "$BASE_DIR/node2/logs/amorphdb.log"

security:
  enable_auth: false  # Simplified for local testing
EOF

# Node 3 - Mesh member
cat > "$BASE_DIR/node3/config.yaml" << EOF
data:
  storage_dir: "$BASE_DIR/node3/data"

network:
  port: $((BASE_PORT+2))
  local_socket_path: "$BASE_DIR/node3/amorphdb.sock"
  max_connections: 100

mesh:
  name: ""  # Will be set when joining mesh
  identity: "node3-member"

logging:
  level: "info"
  file: "$BASE_DIR/node3/logs/amorphdb.log"

security:
  enable_auth: false  # Simplified for local testing
EOF

# Create startup scripts for each node
for i in 1 2 3; do
    cat > "$BASE_DIR/node$i/start.sh" << EOF
#!/bin/bash
cd "$BASE_DIR/node$i"
echo "Starting AmorphDB Node $i..."
exec amorphd --config config.yaml
EOF
    chmod +x "$BASE_DIR/node$i/start.sh"
done

# Create mesh setup script
cat > "$BASE_DIR/setup_mesh.sh" << 'EOF'
#!/bin/bash
set -euo pipefail

echo "🔧 Setting up mesh connections..."

# Wait for Node 1 to start
echo "⏳ Waiting for Node 1 to be ready..."
for i in {1..30}; do
    if amorphctl -s "$BASE_DIR/node1/amorphdb.sock" status &>/dev/null; then
        break
    fi
    sleep 1
done

# Create mesh on Node 1
echo "🏗️  Creating mesh on Node 1..."
amorphctl -s "$BASE_DIR/node1/amorphdb.sock" create-mesh "$MESH_NAME"

# Wait a moment for mesh initialization
sleep 2

# Start Node 2 and join mesh
echo "🔗 Starting Node 2..."
"$BASE_DIR/node2/start.sh" &
NODE2_PID=$!

# Wait for Node 2 to start
echo "⏳ Waiting for Node 2 to be ready..."
for i in {1..30}; do
    if amorphctl -s "$BASE_DIR/node2/amorphdb.sock" status &>/dev/null; then
        break
    fi
    sleep 1
done

# Join mesh from Node 2
echo "🤝 Node 2 joining mesh..."
amorphctl -s "$BASE_DIR/node2/amorphdb.sock" join "127.0.0.1:$BASE_PORT"

# Start Node 3 and join mesh
echo "🔗 Starting Node 3..."
"$BASE_DIR/node3/start.sh" &
NODE3_PID=$!

# Wait for Node 3 to start
echo "⏳ Waiting for Node 3 to be ready..."
for i in {1..30}; do
    if amorphctl -s "$BASE_DIR/node3/amorphdb.sock" status &>/dev/null; then
        break
    fi
    sleep 1
done

# Join mesh from Node 3
echo "🤝 Node 3 joining mesh..."
amorphctl -s "$BASE_DIR/node3/amorphdb.sock" join "127.0.0.1:$BASE_PORT"

# Wait for mesh stabilization
echo "⏳ Waiting for mesh to stabilize..."
sleep 5

# Verify mesh status
echo "📊 Mesh Status:"
echo "Node 1:"
amorphctl -s "$BASE_DIR/node1/amorphdb.sock" status --verbose
echo ""
echo "Node 2:"
amorphctl -s "$BASE_DIR/node2/amorphdb.sock" status --verbose
echo ""
echo "Node 3:"
amorphctl -s "$BASE_DIR/node3/amorphdb.sock" status --verbose

echo ""
echo "✅ Three-node mesh setup complete!"
echo "   Mesh: $MESH_NAME"
echo "   Nodes: 3 active"
echo ""
echo "To interact with the mesh:"
echo "  Node 1: amorphctl -s $BASE_DIR/node1/amorphdb.sock"
echo "  Node 2: amorphctl -s $BASE_DIR/node2/amorphdb.sock"
echo "  Node 3: amorphctl -s $BASE_DIR/node3/amorphdb.sock"
EOF

chmod +x "$BASE_DIR/setup_mesh.sh"

# Create test data script
cat > "$BASE_DIR/test_mesh.sh" << 'EOF'
#!/bin/bash
set -euo pipefail

echo "🧪 Running mesh functionality tests..."

# Test 1: Write data to Node 1, read from Node 2
echo "Test 1: Cross-node data replication"
amorphctl -s "$BASE_DIR/node1/amorphdb.sock" set world.shared.test.message "Hello from Node 1"
sleep 1
RESULT=$(amorphctl -s "$BASE_DIR/node2/amorphdb.sock" get world.shared.test.message)
echo "  Written on Node 1, read on Node 2: $RESULT"

# Test 2: Zone distribution
echo ""
echo "Test 2: Zone distribution"
for i in 1 2 3; do
    echo "Node $i zones:"
    amorphctl -s "$BASE_DIR/node$i/amorphdb.sock" mesh zones | sed 's/^/  /'
done

# Test 3: Mesh members
echo ""
echo "Test 3: Mesh membership"
amorphctl -s "$BASE_DIR/node1/amorphdb.sock" mesh members

# Test 4: Write to different zones
echo ""
echo "Test 4: Multi-zone data distribution"
amorphctl -s "$BASE_DIR/node1/amorphdb.sock" set world.shared.zone_a.data "Zone A data"
amorphctl -s "$BASE_DIR/node2/amorphdb.sock" set world.shared.zone_m.data "Zone M data"
amorphctl -s "$BASE_DIR/node3/amorphdb.sock" set world.shared.zone_z.data "Zone Z data"

echo ""
echo "Reading all zone data from Node 1:"
for zone in zone_a zone_m zone_z; do
    RESULT=$(amorphctl -s "$BASE_DIR/node1/amorphdb.sock" get "world.shared.$zone.data")
    echo "  $zone: $RESULT"
done

echo ""
echo "✅ Mesh tests complete!"
EOF

chmod +x "$BASE_DIR/test_mesh.sh"

# Create management script
cat > "$BASE_DIR/manage.sh" << EOF
#!/bin/bash
set -euo pipefail

case "\${1:-help}" in
    start)
        echo "🚀 Starting three-node mesh..."

        # Start Node 1 (founder)
        echo "Starting Node 1..."
        "$BASE_DIR/node1/start.sh" &
        NODE1_PID=\$!
        echo "Node 1 PID: \$NODE1_PID"

        # Wait and setup mesh
        sleep 3
        "$BASE_DIR/setup_mesh.sh"

        echo "All nodes running. PIDs saved in $BASE_DIR/pids.txt"
        ps aux | grep "amorphd.*$BASE_DIR" | grep -v grep | awk '{print \$2}' > "$BASE_DIR/pids.txt"
        ;;

    stop)
        echo "🛑 Stopping mesh nodes..."
        if [ -f "$BASE_DIR/pids.txt" ]; then
            while read pid; do
                kill "\$pid" 2>/dev/null || true
            done < "$BASE_DIR/pids.txt"
            rm -f "$BASE_DIR/pids.txt"
        fi
        pkill -f "amorphd.*$BASE_DIR" || true
        echo "All nodes stopped."
        ;;

    status)
        echo "📊 Mesh Status:"
        for i in 1 2 3; do
            echo "Node \$i:"
            if amorphctl -s "$BASE_DIR/node\$i/amorphdb.sock" status 2>/dev/null; then
                echo "  ✅ Running"
            else
                echo "  ❌ Not responding"
            fi
        done
        ;;

    test)
        echo "🧪 Running mesh tests..."
        "$BASE_DIR/test_mesh.sh"
        ;;

    logs)
        echo "📝 Following logs (Ctrl+C to stop):"
        tail -f "$BASE_DIR"/node*/logs/amorphdb.log
        ;;

    clean)
        echo "🧹 Cleaning up mesh..."
        "\$0" stop
        rm -rf "$BASE_DIR"
        echo "Cleanup complete."
        ;;

    *)
        echo "AmorphDB Three-Node Mesh Management"
        echo ""
        echo "Usage: \$0 {start|stop|status|test|logs|clean}"
        echo ""
        echo "Commands:"
        echo "  start   - Start all three nodes and create mesh"
        echo "  stop    - Stop all nodes"
        echo "  status  - Check node status"
        echo "  test    - Run mesh functionality tests"
        echo "  logs    - Follow all node logs"
        echo "  clean   - Stop and remove all data"
        echo ""
        echo "Mesh: $MESH_NAME"
        echo "Directory: $BASE_DIR"
        ;;
esac
EOF

chmod +x "$BASE_DIR/manage.sh"

echo "✅ Three-node mesh setup complete!"
echo ""
echo "To start the mesh:"
echo "  $BASE_DIR/manage.sh start"
echo ""
echo "To manage the mesh:"
echo "  $BASE_DIR/manage.sh {start|stop|status|test|logs|clean}"
echo ""
echo "Setup directory: $BASE_DIR"