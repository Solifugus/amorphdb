#!/bin/bash
# AmorphDB Release Builder
# Creates distribution packages for multiple platforms

set -euo pipefail

# Configuration
VERSION="${VERSION:-$(date +%Y.%m.%d)}"
BUILD_DIR="./dist"
RELEASE_DIR="$BUILD_DIR/release-$VERSION"

# Platforms to build for
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# Clean and create build directory
log_info "Preparing release build for version $VERSION"
rm -rf "$BUILD_DIR"
mkdir -p "$RELEASE_DIR"

# Build binaries for each platform
for platform in "${PLATFORMS[@]}"; do
    os="${platform%/*}"
    arch="${platform#*/}"

    log_info "Building for $os/$arch"

    # Create platform directory
    platform_dir="$RELEASE_DIR/amorphdb-$VERSION-$os-$arch"
    mkdir -p "$platform_dir/bin"
    mkdir -p "$platform_dir/config"
    mkdir -p "$platform_dir/docs"
    mkdir -p "$platform_dir/examples"
    mkdir -p "$platform_dir/scripts"

    # Build binaries
    if [ "$os" = "windows" ]; then
        ext=".exe"
    else
        ext=""
    fi

    GOOS=$os GOARCH=$arch go build -ldflags="-s -w" -o "$platform_dir/bin/amorphd$ext" ./cmd/amorphd
    GOOS=$os GOARCH=$arch go build -ldflags="-s -w" -o "$platform_dir/bin/amorph$ext" ./cmd/amorph
    GOOS=$os GOARCH=$arch go build -ldflags="-s -w" -o "$platform_dir/bin/amorphctl$ext" ./cmd/amorphctl

    # Copy documentation and examples
    cp -r docs/* "$platform_dir/docs/"
    cp -r examples/* "$platform_dir/examples/"
    cp README.md "$platform_dir/"
    cp INSTALL.md "$platform_dir/"
    cp LICENSE "$platform_dir/" 2>/dev/null || echo "# Commercial License" > "$platform_dir/LICENSE"

    # Copy configuration examples
    cp examples/mesh_setup/configs/*.yaml "$platform_dir/config/"

    # Create platform-specific install script
    create_install_script "$platform_dir" "$os"

    # Create archive
    cd "$RELEASE_DIR"
    if [ "$os" = "windows" ]; then
        zip -r "amorphdb-$VERSION-$os-$arch.zip" "amorphdb-$VERSION-$os-$arch/"
    else
        tar -czf "amorphdb-$VERSION-$os-$arch.tar.gz" "amorphdb-$VERSION-$os-$arch/"
    fi
    cd - > /dev/null

    # Clean up directory after archiving
    rm -rf "$platform_dir"

    log_success "Built $os/$arch"
done

# Create checksums
log_info "Creating checksums..."
cd "$RELEASE_DIR"
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum *.tar.gz *.zip > SHA256SUMS 2>/dev/null || true
elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 *.tar.gz *.zip > SHA256SUMS 2>/dev/null || true
fi
cd - > /dev/null

log_success "Release build complete: $RELEASE_DIR"
echo ""
echo "Distribution packages:"
ls -la "$RELEASE_DIR"

# Function to create platform-specific install scripts
create_install_script() {
    local dir="$1"
    local os="$2"

    if [ "$os" = "windows" ]; then
        # Windows PowerShell install script
        cat > "$dir/install.ps1" << 'EOF'
# AmorphDB Windows Installation Script
param(
    [string]$InstallPath = "$env:ProgramFiles\AmorphDB",
    [switch]$AddToPath
)

Write-Host "Installing AmorphDB for Windows..." -ForegroundColor Blue

# Create installation directory
if (!(Test-Path $InstallPath)) {
    New-Item -ItemType Directory -Path $InstallPath -Force | Out-Null
}

# Copy binaries
Copy-Item "bin\*" -Destination $InstallPath -Recurse -Force

# Create data directory
$DataPath = "$env:ProgramData\AmorphDB"
$LogPath = "$DataPath\logs"
if (!(Test-Path $DataPath)) {
    New-Item -ItemType Directory -Path $DataPath -Force | Out-Null
    New-Item -ItemType Directory -Path $LogPath -Force | Out-Null
}

# Create Windows-specific config
$ConfigContent = @"
# AmorphDB Windows Configuration
data:
  storage_dir: "$DataPath"

network:
  port: 8080
  # local_socket_path disabled on Windows - using TCP only
  max_connections: 1000
  bind_address: "127.0.0.1"  # Localhost only for security

mesh:
  name: ""
  identity: "$env:COMPUTERNAME"

logging:
  level: "info"
  file: "$LogPath\amorphdb.log"

security:
  enable_auth: false  # Enable for production
"@

$ConfigContent | Out-File -FilePath "$DataPath\amorphd.yaml" -Encoding UTF8

Write-Host "AmorphDB installed to: $InstallPath" -ForegroundColor Green

if ($AddToPath) {
    # Add to PATH (requires admin)
    try {
        $currentPath = [Environment]::GetEnvironmentVariable("PATH", "Machine")
        if ($currentPath -notlike "*$InstallPath*") {
            [Environment]::SetEnvironmentVariable("PATH", "$currentPath;$InstallPath", "Machine")
            Write-Host "Added to system PATH (restart required)" -ForegroundColor Green
        }
    } catch {
        Write-Host "Could not add to PATH (requires admin privileges)" -ForegroundColor Yellow
        Write-Host "Add manually: $InstallPath" -ForegroundColor Yellow
    }
}

# Create start script
$StartScript = @"
@echo off
cd /d "$InstallPath"
echo Starting AmorphDB for Windows...
echo Connect with: amorph.exe --host=localhost:8080
echo Web interface: http://localhost:8080
echo.
amorphd.exe --config="$DataPath\amorphd.yaml"
"@

$StartScript | Out-File -FilePath "$InstallPath\start.bat" -Encoding ASCII

Write-Host ""
Write-Host "IMPORTANT: Windows uses TCP-only mode (no local sockets)" -ForegroundColor Yellow
Write-Host ""
Write-Host "Quick Start:" -ForegroundColor Cyan
Write-Host "  Option 1: Double-click start.bat in $InstallPath"
Write-Host "  Option 2: Manual start:"
Write-Host "    cd `"$InstallPath`""
Write-Host "    .\amorphd.exe --config=`"$DataPath\amorphd.yaml`""
Write-Host "    (new window) .\amorph.exe --host=localhost:8080"
Write-Host ""
Write-Host "Documentation: docs\"
EOF

        # Also create a simple batch file installer
        cat > "$dir/install.bat" << 'EOF'
@echo off
echo Installing AmorphDB for Windows...

set INSTALL_DIR=%USERPROFILE%\AmorphDB
set DATA_DIR=%PROGRAMDATA%\AmorphDB

echo Creating directories...
mkdir "%INSTALL_DIR%" 2>nul
mkdir "%DATA_DIR%" 2>nul
mkdir "%DATA_DIR%\logs" 2>nul

echo Copying binaries...
copy bin\*.exe "%INSTALL_DIR%"

echo Creating configuration...
echo # AmorphDB Windows Configuration > "%DATA_DIR%\amorphd.yaml"
echo data: >> "%DATA_DIR%\amorphd.yaml"
echo   storage_dir: "%DATA_DIR%" >> "%DATA_DIR%\amorphd.yaml"
echo network: >> "%DATA_DIR%\amorphd.yaml"
echo   port: 8080 >> "%DATA_DIR%\amorphd.yaml"
echo   bind_address: "127.0.0.1" >> "%DATA_DIR%\amorphd.yaml"
echo mesh: >> "%DATA_DIR%\amorphd.yaml"
echo   name: "" >> "%DATA_DIR%\amorphd.yaml"
echo   identity: "%COMPUTERNAME%" >> "%DATA_DIR%\amorphd.yaml"
echo logging: >> "%DATA_DIR%\amorphd.yaml"
echo   level: "info" >> "%DATA_DIR%\amorphd.yaml"
echo   file: "%DATA_DIR%\logs\amorphdb.log" >> "%DATA_DIR%\amorphd.yaml"

echo.
echo ============================================
echo AmorphDB installed successfully!
echo ============================================
echo.
echo Installation: %INSTALL_DIR%
echo Data: %DATA_DIR%
echo.
echo IMPORTANT: Windows uses TCP-only mode
echo.
echo Quick Start:
echo   cd "%INSTALL_DIR%"
echo   amorphd.exe --config="%DATA_DIR%\amorphd.yaml"
echo   (new window) amorph.exe --host=localhost:8080
echo.
echo Add to PATH: set PATH=%INSTALL_DIR%;%%PATH%%
echo.
pause
EOF

    else
        # Unix install script
        cat > "$dir/install.sh" << 'EOF'
#!/bin/bash
# AmorphDB Binary Installation Script

set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
DATA_DIR="${DATA_DIR:-/var/lib/amorphdb}"
CONFIG_DIR="${CONFIG_DIR:-/etc/amorphdb}"
CREATE_SERVICE="${CREATE_SERVICE:-true}"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

echo "Installing AmorphDB..."

# Check for required permissions
if [[ "$CREATE_SERVICE" == "true" && $EUID -ne 0 ]]; then
    log_error "System installation requires sudo. Run with sudo, or use:"
    echo "  INSTALL_DIR=~/.local/bin CREATE_SERVICE=false ./install.sh"
    exit 1
fi

# Install binaries
log_info "Installing binaries to $INSTALL_DIR"
mkdir -p "$INSTALL_DIR"
cp bin/* "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR"/{amorphd,amorph,amorphctl}

if [[ "$CREATE_SERVICE" == "true" ]]; then
    # Create system directories
    mkdir -p "$DATA_DIR" "$CONFIG_DIR"

    # Create amorphdb user
    if ! id amorphdb >/dev/null 2>&1; then
        useradd --system --home-dir "$DATA_DIR" --shell /bin/false amorphdb
    fi

    chown amorphdb:amorphdb "$DATA_DIR"

    # Install config
    if [[ ! -f "$CONFIG_DIR/amorphd.yaml" ]]; then
        cp config/basic_config.yaml "$CONFIG_DIR/amorphd.yaml"
        # Update paths in config
        sed -i "s|/var/lib/amorphdb|$DATA_DIR|g" "$CONFIG_DIR/amorphd.yaml"
        chown root:amorphdb "$CONFIG_DIR/amorphd.yaml"
        chmod 640 "$CONFIG_DIR/amorphd.yaml"
    fi

    # Create systemd service
    if command -v systemctl >/dev/null 2>&1; then
        cat > /etc/systemd/system/amorphd.service << SYSTEMD_EOF
[Unit]
Description=AmorphDB Temporal Database
After=network.target

[Service]
Type=simple
User=amorphdb
Group=amorphdb
ExecStart=$INSTALL_DIR/amorphd --config=$CONFIG_DIR/amorphd.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
SYSTEMD_EOF

        systemctl daemon-reload
        systemctl enable amorphd
        log_success "Systemd service created and enabled"
    fi
fi

log_success "AmorphDB installation complete!"
echo ""
echo "Quick Start:"
if [[ "$CREATE_SERVICE" == "true" ]]; then
    echo "  sudo systemctl start amorphd"
    echo "  amorphctl status"
else
    echo "  amorphd &"
    echo "  amorphctl status"
fi
echo ""
echo "Documentation: docs/"
echo "Examples: examples/"
EOF
        chmod +x "$dir/install.sh"
    fi
}