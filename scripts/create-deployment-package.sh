#!/bin/bash
# Simple Deployment Package Creator
# Creates a minimal deployment package with just the essentials

set -euo pipefail

# Configuration
VERSION="${VERSION:-$(date +%Y.%m.%d)}"
PACKAGE_NAME="amorphdb-deployment-$VERSION"
OUTPUT_DIR="./dist/$PACKAGE_NAME"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }

# Create deployment package
create_package() {
    log_info "Creating deployment package: $PACKAGE_NAME"

    # Clean and create output directory
    rm -rf "$OUTPUT_DIR"
    mkdir -p "$OUTPUT_DIR"/{bin,config,docs,scripts}

    # Copy binaries (must be built first)
    if [[ ! -f "./bin/amorphd" ]]; then
        log_info "Building binaries first..."
        make build
    fi

    cp ./bin/* "$OUTPUT_DIR/bin/"
    chmod +x "$OUTPUT_DIR/bin"/*

    # Copy essential documentation
    cp README.md INSTALL.md DISTRIBUTION.md "$OUTPUT_DIR/"
    cp docs/mesh_management_guide.md "$OUTPUT_DIR/docs/"
    cp docs/AmorphDB_Tutorial.md "$OUTPUT_DIR/docs/"
    cp -r examples "$OUTPUT_DIR/"

    # Copy configuration examples
    cp examples/mesh_setup/configs/*.yaml "$OUTPUT_DIR/config/"

    # Create simple install script
    cat > "$OUTPUT_DIR/install.sh" << 'EOF'
#!/bin/bash
# AmorphDB Simple Installation Script

INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
CONFIG_DIR="${CONFIG_DIR:-$HOME/.config/amorphdb}"

echo "Installing AmorphDB to $INSTALL_DIR"

# Create directories
mkdir -p "$INSTALL_DIR" "$CONFIG_DIR"

# Copy binaries
cp bin/* "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR"/{amorphd,amorph,amorphctl}

# Copy default config
cp config/basic_config.yaml "$CONFIG_DIR/amorphd.yaml"

echo ""
echo "✅ AmorphDB installed successfully!"
echo ""
echo "Add to PATH: export PATH=\"$INSTALL_DIR:\$PATH\""
echo "Start: amorphd --config=\"$CONFIG_DIR/amorphd.yaml\" &"
echo "Connect: amorph"
echo ""
echo "Documentation: docs/"
echo "Examples: examples/"
EOF

    chmod +x "$OUTPUT_DIR/install.sh"

    # Create uninstall script
    cat > "$OUTPUT_DIR/uninstall.sh" << 'EOF'
#!/bin/bash
# AmorphDB Uninstall Script

INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
CONFIG_DIR="${CONFIG_DIR:-$HOME/.config/amorphdb}"

echo "Removing AmorphDB binaries from $INSTALL_DIR"
rm -f "$INSTALL_DIR"/{amorphd,amorph,amorphctl}

echo "Configuration preserved in $CONFIG_DIR"
echo "To remove config: rm -rf \"$CONFIG_DIR\""
echo ""
echo "✅ AmorphDB uninstalled"
EOF

    chmod +x "$OUTPUT_DIR/uninstall.sh"

    # Create Windows install script
    cat > "$OUTPUT_DIR/install.ps1" << 'EOF'
# AmorphDB Windows Installation
param([string]$InstallPath = "$env:USERPROFILE\AmorphDB")

Write-Host "Installing AmorphDB to $InstallPath"
New-Item -ItemType Directory -Path $InstallPath -Force | Out-Null
Copy-Item "bin\*" -Destination $InstallPath
$configDir = "$env:USERPROFILE\.amorphdb"
New-Item -ItemType Directory -Path $configDir -Force | Out-Null
Copy-Item "config\basic_config.yaml" -Destination "$configDir\amorphd.yaml"

Write-Host ""
Write-Host "AmorphDB installed successfully!" -ForegroundColor Green
Write-Host "Start: cd `"$InstallPath`" && .\amorphd.exe"
EOF

    # Create README for the package
    cat > "$OUTPUT_DIR/README_FIRST.md" << EOF
# AmorphDB Deployment Package v$VERSION

This package contains pre-built AmorphDB binaries and everything needed for deployment.

## Quick Start

### Linux/macOS
\`\`\`bash
./install.sh
export PATH="\$HOME/.local/bin:\$PATH"
amorphd &
amorph
\`\`\`

### Windows
\`\`\`powershell
.\install.ps1
cd \$env:USERPROFILE\AmorphDB
.\amorphd.exe
\`\`\`

## What's Included

- **bin/** - AmorphDB binaries (amorphd, amorph, amorphctl)
- **config/** - Configuration examples
- **docs/** - Essential documentation
- **examples/** - Usage examples and workflows
- **install.sh** - Installation script for Unix systems
- **install.ps1** - Installation script for Windows
- **uninstall.sh** - Removal script

## Documentation

- **README.md** - Main project overview
- **INSTALL.md** - Complete installation guide
- **docs/mesh_management_guide.md** - Mesh operations guide
- **docs/AmorphDB_Tutorial.md** - Complete tutorial

## Support

- Documentation: docs/
- Examples: examples/
- Website: https://amorphdb.com
- Support: support@amorphdb.com

---

AmorphDB v$VERSION - Temporal Database with Multi-Mesh Architecture
EOF
}

# Create archive
create_archive() {
    log_info "Creating archive..."

    cd "$(dirname "$OUTPUT_DIR")"
    tar -czf "$PACKAGE_NAME.tar.gz" "$PACKAGE_NAME/"
    zip -r "$PACKAGE_NAME.zip" "$PACKAGE_NAME/" >/dev/null

    # Create checksums
    sha256sum "$PACKAGE_NAME.tar.gz" "$PACKAGE_NAME.zip" > "$PACKAGE_NAME.SHA256"

    cd - >/dev/null

    log_success "Archive created: $(dirname "$OUTPUT_DIR")/$PACKAGE_NAME.{tar.gz,zip}"
}

# Main execution
main() {
    create_package
    create_archive

    echo ""
    log_success "Deployment package ready!"
    echo ""
    echo "Package contents:"
    ls -la "$OUTPUT_DIR"
    echo ""
    echo "Archives:"
    ls -la "$(dirname "$OUTPUT_DIR")/$PACKAGE_NAME".*
    echo ""
    echo "Distribution ready for customers!"
}

main "$@"