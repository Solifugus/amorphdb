#!/bin/bash
# AmorphDB Standalone Development Setup
#
# This script sets up a single AmorphDB node for development work.
# Perfect for testing MBL scripts and learning the system.

set -euo pipefail

# Configuration
AMORPHDB_DIR="${AMORPHDB_DIR:-$HOME/.amorphdb-dev}"
PORT="${PORT:-8080}"
LOG_LEVEL="${LOG_LEVEL:-info}"

echo "🚀 Setting up AmorphDB standalone development node..."
echo "   Directory: $AMORPHDB_DIR"
echo "   Port: $PORT"

# Create data directory
mkdir -p "$AMORPHDB_DIR/data"
mkdir -p "$AMORPHDB_DIR/logs"

# Generate development configuration
cat > "$AMORPHDB_DIR/config.yaml" << EOF
# AmorphDB Development Configuration
data:
  storage_dir: "$AMORPHDB_DIR/data"

network:
  port: $PORT
  local_socket_path: "$AMORPHDB_DIR/amorphdb.sock"
  max_connections: 50

mesh:
  name: ""  # Standalone mode
  identity: "dev-node-$(whoami)"

logging:
  level: "$LOG_LEVEL"
  file: "$AMORPHDB_DIR/logs/amorphdb.log"

security:
  enable_auth: false  # Development only

development:
  enable_debug_endpoints: true
  auto_save_interval: 10s
EOF

# Create development data
cat > "$AMORPHDB_DIR/sample_data.mbl" << 'EOF'
# Sample development data for testing
# Use 'amorphctl exec' to run these commands

# Create some sample data structures
local.users.admin.name = "Development Admin"
local.users.admin.email = "admin@example.com"
local.users.admin.created = now()

local.projects.demo.name = "Demo Project"
local.projects.demo.status = "active"
local.projects.demo.created = now()

# Create some sample procedures
procedure local.procedures.hello {
    return "Hello from AmorphDB!"
}

procedure local.procedures.user_count {
    return count(local.users)
}

# Set up some watchers for development
watch local.users.* {
    log("User data changed: " + path)
}
EOF

# Create startup script
cat > "$AMORPHDB_DIR/start.sh" << EOF
#!/bin/bash
cd "$AMORPHDB_DIR"
echo "Starting AmorphDB development server..."
echo "Configuration: $AMORPHDB_DIR/config.yaml"
echo "Local socket: $AMORPHDB_DIR/amorphdb.sock"
echo "Web interface: http://localhost:$PORT"
echo ""
echo "Quick commands:"
echo "  amorphctl -s $AMORPHDB_DIR/amorphdb.sock status"
echo "  amorphctl -s $AMORPHDB_DIR/amorphdb.sock exec < sample_data.mbl"
echo "  amorphctl -s $AMORPHDB_DIR/amorphdb.sock get local.users"
echo ""

exec amorphd --config "$AMORPHDB_DIR/config.yaml"
EOF

chmod +x "$AMORPHDB_DIR/start.sh"

# Create helper scripts
cat > "$AMORPHDB_DIR/console.sh" << EOF
#!/bin/bash
# Interactive console for development
amorphctl -s "$AMORPHDB_DIR/amorphdb.sock" console
EOF

chmod +x "$AMORPHDB_DIR/console.sh"

cat > "$AMORPHDB_DIR/load_sample_data.sh" << EOF
#!/bin/bash
# Load sample data into development instance
amorphctl -s "$AMORPHDB_DIR/amorphdb.sock" exec < "$AMORPHDB_DIR/sample_data.mbl"
echo "Sample data loaded successfully!"
EOF

chmod +x "$AMORPHDB_DIR/load_sample_data.sh"

# Create development aliases
cat > "$AMORPHDB_DIR/aliases.sh" << EOF
# Development aliases - source this file in your shell
alias adb="amorphctl -s $AMORPHDB_DIR/amorphdb.sock"
alias adb-status="adb status"
alias adb-console="adb console"
alias adb-get="adb get"
alias adb-set="adb set"
alias adb-logs="tail -f $AMORPHDB_DIR/logs/amorphdb.log"

echo "AmorphDB development aliases loaded:"
echo "  adb         - AmorphDB control (short form)"
echo "  adb-status  - Check server status"
echo "  adb-console - Interactive console"
echo "  adb-get     - Get data"
echo "  adb-set     - Set data"
echo "  adb-logs    - Follow server logs"
EOF

echo "✅ Development environment configured!"
echo ""
echo "To start your development server:"
echo "  $AMORPHDB_DIR/start.sh"
echo ""
echo "To load development aliases:"
echo "  source $AMORPHDB_DIR/aliases.sh"
echo ""
echo "To load sample data (after starting server):"
echo "  $AMORPHDB_DIR/load_sample_data.sh"
echo ""
echo "Development directory: $AMORPHDB_DIR"