#!/bin/bash
# AmorphDB Installation Script
# Provides system-wide installation with proper service setup

set -euo pipefail

# Configuration
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
DATA_DIR="${DATA_DIR:-/var/lib/amorphdb}"
CONFIG_DIR="${CONFIG_DIR:-/etc/amorphdb}"
LOG_DIR="${LOG_DIR:-/var/log/amorphdb}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
USER="${AMORPHDB_USER:-amorphdb}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

check_dependencies() {
    log_info "Checking dependencies..."

    # Check for required commands
    local deps=("systemctl" "useradd" "mkdir" "chown" "chmod")
    for dep in "${deps[@]}"; do
        if ! command -v "$dep" &> /dev/null; then
            log_error "Required command '$dep' not found"
            exit 1
        fi
    done

    log_success "Dependencies check passed"
}

build_binaries() {
    log_info "Building AmorphDB binaries..."

    if [[ ! -f "Makefile" ]]; then
        log_error "Makefile not found. Are you in the AmorphDB repository root?"
        exit 1
    fi

    make build

    # Verify binaries were built
    local binaries=("./bin/amorphd" "./bin/amorph" "./bin/amorphctl")
    for binary in "${binaries[@]}"; do
        if [[ ! -f "$binary" ]]; then
            log_error "Binary $binary not found after build"
            exit 1
        fi
    done

    log_success "Binaries built successfully"
}

create_user() {
    log_info "Creating AmorphDB user..."

    if id "$USER" &>/dev/null; then
        log_warning "User $USER already exists"
    else
        useradd --system --home-dir "$DATA_DIR" --shell /bin/false --user-group "$USER"
        log_success "Created user $USER"
    fi
}

create_directories() {
    log_info "Creating directories..."

    local dirs=("$DATA_DIR" "$CONFIG_DIR" "$LOG_DIR")
    for dir in "${dirs[@]}"; do
        mkdir -p "$dir"
        chown "$USER:$USER" "$dir"
        chmod 755 "$dir"
        log_success "Created directory $dir"
    done
}

install_binaries() {
    log_info "Installing binaries to $INSTALL_DIR..."

    local binaries=("amorphd" "amorph" "amorphctl")
    for binary in "${binaries[@]}"; do
        cp "./bin/$binary" "$INSTALL_DIR/"
        chmod +x "$INSTALL_DIR/$binary"
        chown root:root "$INSTALL_DIR/$binary"
        log_success "Installed $binary"
    done
}

create_config() {
    log_info "Creating default configuration..."

    cat > "$CONFIG_DIR/amorphd.yaml" << EOF
# AmorphDB Configuration
data:
  storage_dir: "$DATA_DIR"

network:
  port: 8080
  local_socket_path: "/var/run/amorphdb.sock"
  max_connections: 1000
  bind_address: "0.0.0.0"

mesh:
  name: ""                    # Empty for standalone mode
  identity: "$(hostname)"     # Node identifier

logging:
  level: "info"
  file: "$LOG_DIR/amorphdb.log"
  max_size: "100MB"
  max_backups: 10
  compress: true

security:
  enable_auth: false          # Enable in production
EOF

    chown "$USER:$USER" "$CONFIG_DIR/amorphd.yaml"
    chmod 640 "$CONFIG_DIR/amorphd.yaml"
    log_success "Created configuration file"
}

create_systemd_service() {
    log_info "Creating systemd service..."

    cat > "$SYSTEMD_DIR/amorphd.service" << EOF
[Unit]
Description=AmorphDB Temporal Database
Documentation=https://github.com/solifugus/amorphdb
After=network.target

[Service]
Type=simple
User=$USER
Group=$USER
ExecStart=$INSTALL_DIR/amorphd --config=$CONFIG_DIR/amorphd.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=amorphd

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$DATA_DIR $LOG_DIR /var/run

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF

    chmod 644 "$SYSTEMD_DIR/amorphd.service"
    log_success "Created systemd service"
}

setup_logrotate() {
    log_info "Setting up log rotation..."

    cat > "/etc/logrotate.d/amorphdb" << EOF
$LOG_DIR/*.log {
    daily
    missingok
    rotate 30
    compress
    delaycompress
    notifempty
    create 644 $USER $USER
    postrotate
        systemctl reload-or-restart amorphd
    endscript
}
EOF

    log_success "Set up log rotation"
}

enable_service() {
    log_info "Enabling AmorphDB service..."

    systemctl daemon-reload
    systemctl enable amorphd.service
    log_success "Service enabled"
}

show_completion() {
    log_success "AmorphDB installation completed!"
    echo ""
    echo "📋 Installation Summary:"
    echo "  Binaries: $INSTALL_DIR/{amorphd,amorph,amorphctl}"
    echo "  Data: $DATA_DIR"
    echo "  Config: $CONFIG_DIR/amorphd.yaml"
    echo "  Logs: $LOG_DIR/"
    echo "  Service: amorphd.service"
    echo ""
    echo "🚀 Quick Start:"
    echo "  sudo systemctl start amorphd     # Start the service"
    echo "  sudo systemctl status amorphd    # Check service status"
    echo "  amorph                           # Connect with client"
    echo "  amorphctl status                 # Check database status"
    echo ""
    echo "📖 Documentation:"
    echo "  Mesh Guide: docs/mesh_management_guide.md"
    echo "  Tutorial: docs/AmorphDB_Tutorial.md"
    echo "  Examples: examples/"
    echo ""
    echo "⚙️  Next Steps:"
    echo "  1. Review configuration in $CONFIG_DIR/amorphd.yaml"
    echo "  2. Start the service: sudo systemctl start amorphd"
    echo "  3. Create your first mesh: amorphctl create-mesh 'my-mesh'"
}

# Main installation process
main() {
    log_info "Starting AmorphDB installation..."
    echo ""

    check_root
    check_dependencies
    build_binaries
    create_user
    create_directories
    install_binaries
    create_config
    create_systemd_service
    setup_logrotate
    enable_service

    echo ""
    show_completion
}

# Handle command line arguments
case "${1:-install}" in
    install)
        main
        ;;
    uninstall)
        log_info "Uninstalling AmorphDB..."
        systemctl stop amorphd.service 2>/dev/null || true
        systemctl disable amorphd.service 2>/dev/null || true
        rm -f "$SYSTEMD_DIR/amorphd.service"
        rm -f "$INSTALL_DIR"/{amorphd,amorph,amorphctl}
        rm -f "/etc/logrotate.d/amorphdb"
        systemctl daemon-reload
        log_warning "User '$USER' and data directories were NOT removed for safety"
        log_warning "To remove completely: sudo userdel $USER && sudo rm -rf $DATA_DIR $CONFIG_DIR $LOG_DIR"
        log_success "AmorphDB uninstalled"
        ;;
    --help|-h)
        echo "AmorphDB Installation Script"
        echo ""
        echo "Usage: $0 [install|uninstall|--help]"
        echo ""
        echo "Environment variables:"
        echo "  INSTALL_DIR    Installation directory (default: /usr/local/bin)"
        echo "  DATA_DIR       Data directory (default: /var/lib/amorphdb)"
        echo "  CONFIG_DIR     Config directory (default: /etc/amorphdb)"
        echo "  LOG_DIR        Log directory (default: /var/log/amorphdb)"
        echo "  AMORPHDB_USER  Service user (default: amorphdb)"
        echo ""
        echo "Example:"
        echo "  sudo ./scripts/install.sh"
        echo "  sudo INSTALL_DIR=/opt/amorphdb ./scripts/install.sh"
        ;;
    *)
        log_error "Unknown command: $1"
        echo "Use --help for usage information"
        exit 1
        ;;
esac