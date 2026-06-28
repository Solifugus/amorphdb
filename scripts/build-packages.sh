#!/bin/bash
# AmorphDB Package Builder
# Creates .deb and .rpm packages for Linux distributions

set -euo pipefail

# Configuration
VERSION="${VERSION:-$(date +%Y.%m.%d)}"
PACKAGE_DIR="./dist/packages"
MAINTAINER="${MAINTAINER:-AmorphDB Team <support@example.com>}"
HOMEPAGE="${HOMEPAGE:-https://amorphdb.com}"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }

# Check dependencies
check_deps() {
    local missing=()

    if ! command -v fpm >/dev/null 2>&1; then
        missing+=("fpm")
    fi

    if [ ${#missing[@]} -ne 0 ]; then
        echo "Missing dependencies: ${missing[*]}"
        echo ""
        echo "Install with:"
        echo "  gem install fpm  # or: sudo apt install ruby-dev build-essential && gem install fpm"
        exit 1
    fi
}

# Build binaries first
build_binaries() {
    log_info "Building binaries for Linux amd64..."
    make build
}

# Create package staging area
create_staging() {
    local staging_dir="$1"

    log_info "Creating package staging area..."
    rm -rf "$staging_dir"
    mkdir -p "$staging_dir"/{usr/local/bin,etc/amorphdb,var/lib/amorphdb,usr/share/doc/amorphdb,etc/systemd/system,etc/logrotate.d}

    # Install binaries
    cp bin/* "$staging_dir/usr/local/bin/"
    chmod +x "$staging_dir/usr/local/bin"/*

    # Install configuration
    cp examples/mesh_setup/configs/basic_config.yaml "$staging_dir/etc/amorphdb/amorphd.yaml"

    # Install documentation
    cp README.md INSTALL.md "$staging_dir/usr/share/doc/amorphdb/"
    cp -r docs/* "$staging_dir/usr/share/doc/amorphdb/"

    # Create systemd service
    cat > "$staging_dir/etc/systemd/system/amorphd.service" << 'EOF'
[Unit]
Description=AmorphDB Temporal Database
Documentation=https://amorphdb.com
After=network.target

[Service]
Type=simple
User=amorphdb
Group=amorphdb
ExecStart=/usr/local/bin/amorphd --config=/etc/amorphdb/amorphd.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/amorphdb /var/log/amorphdb

[Install]
WantedBy=multi-user.target
EOF

    # Create logrotate config
    cat > "$staging_dir/etc/logrotate.d/amorphdb" << 'EOF'
/var/log/amorphdb/*.log {
    daily
    missingok
    rotate 30
    compress
    delaycompress
    notifempty
    create 644 amorphdb amorphdb
    postrotate
        systemctl reload-or-restart amorphd
    endscript
}
EOF
}

# Build .deb package
build_deb() {
    local staging_dir="$PACKAGE_DIR/staging-deb"
    create_staging "$staging_dir"

    log_info "Building .deb package..."

    fpm -s dir -t deb \
        --name amorphdb \
        --version "$VERSION" \
        --maintainer "$MAINTAINER" \
        --description "AmorphDB Temporal Database with Multi-Mesh Bridge Architecture" \
        --long-description "AmorphDB is a revolutionary temporal database that preserves complete data history while enabling secure cross-mesh data collaboration through bridge connections." \
        --url "$HOMEPAGE" \
        --license "Commercial" \
        --category "database" \
        --depends "systemd" \
        --deb-systemd etc/systemd/system/amorphd.service \
        --before-install "$PACKAGE_DIR/scripts/preinst.sh" \
        --after-install "$PACKAGE_DIR/scripts/postinst.sh" \
        --before-remove "$PACKAGE_DIR/scripts/prerm.sh" \
        --after-remove "$PACKAGE_DIR/scripts/postrm.sh" \
        --package "$PACKAGE_DIR" \
        --chdir "$staging_dir" \
        .

    log_success "Created .deb package"
}

# Build .rpm package
build_rpm() {
    local staging_dir="$PACKAGE_DIR/staging-rpm"
    create_staging "$staging_dir"

    log_info "Building .rpm package..."

    fpm -s dir -t rpm \
        --name amorphdb \
        --version "$VERSION" \
        --maintainer "$MAINTAINER" \
        --description "AmorphDB Temporal Database with Multi-Mesh Bridge Architecture" \
        --url "$HOMEPAGE" \
        --license "Commercial" \
        --category "Applications/Databases" \
        --depends "systemd" \
        --rpm-systemd etc/systemd/system/amorphd.service \
        --before-install "$PACKAGE_DIR/scripts/preinst.sh" \
        --after-install "$PACKAGE_DIR/scripts/postinst.sh" \
        --before-remove "$PACKAGE_DIR/scripts/prerm.sh" \
        --after-remove "$PACKAGE_DIR/scripts/postrm.sh" \
        --package "$PACKAGE_DIR" \
        --chdir "$staging_dir" \
        .

    log_success "Created .rpm package"
}

# Create package scripts
create_package_scripts() {
    local scripts_dir="$PACKAGE_DIR/scripts"
    mkdir -p "$scripts_dir"

    # Pre-installation script
    cat > "$scripts_dir/preinst.sh" << 'EOF'
#!/bin/bash
# Create amorphdb user
if ! id amorphdb >/dev/null 2>&1; then
    useradd --system --home-dir /var/lib/amorphdb --shell /bin/false --user-group amorphdb
fi
EOF

    # Post-installation script
    cat > "$scripts_dir/postinst.sh" << 'EOF'
#!/bin/bash
# Set up directories and permissions
mkdir -p /var/lib/amorphdb /var/log/amorphdb
chown amorphdb:amorphdb /var/lib/amorphdb /var/log/amorphdb
chown root:amorphdb /etc/amorphdb/amorphd.yaml
chmod 640 /etc/amorphdb/amorphd.yaml

# Enable and start service
systemctl daemon-reload
systemctl enable amorphd
if systemctl is-active --quiet amorphd; then
    systemctl restart amorphd
fi

echo ""
echo "AmorphDB installed successfully!"
echo ""
echo "Quick start:"
echo "  sudo systemctl start amorphd"
echo "  amorphctl status"
echo ""
echo "Documentation: /usr/share/doc/amorphdb/"
EOF

    # Pre-removal script
    cat > "$scripts_dir/prerm.sh" << 'EOF'
#!/bin/bash
if systemctl is-active --quiet amorphd; then
    systemctl stop amorphd
fi
if systemctl is-enabled --quiet amorphd; then
    systemctl disable amorphd
fi
EOF

    # Post-removal script
    cat > "$scripts_dir/postrm.sh" << 'EOF'
#!/bin/bash
systemctl daemon-reload
# Note: We don't remove user or data for safety
echo "AmorphDB removed. User 'amorphdb' and data in /var/lib/amorphdb preserved."
EOF

    chmod +x "$scripts_dir"/*.sh
}

# Main execution
main() {
    log_info "Building AmorphDB packages version $VERSION"

    check_deps
    build_binaries

    mkdir -p "$PACKAGE_DIR"
    create_package_scripts

    build_deb
    build_rpm

    # Create checksums
    cd "$PACKAGE_DIR"
    sha256sum *.deb *.rpm > SHA256SUMS
    cd - >/dev/null

    echo ""
    log_success "Package build complete!"
    echo ""
    echo "Packages created:"
    ls -la "$PACKAGE_DIR"/*.deb "$PACKAGE_DIR"/*.rpm
    echo ""
    echo "Installation:"
    echo "  Ubuntu/Debian: sudo dpkg -i $PACKAGE_DIR/amorphdb_$VERSION*.deb"
    echo "  RHEL/CentOS:   sudo rpm -i $PACKAGE_DIR/amorphdb-$VERSION*.rpm"
}

main "$@"