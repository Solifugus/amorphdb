# AmorphDB Installation Guide

This guide covers different ways to install and deploy AmorphDB, from development setups to production environments.

## Quick Start

### 1. Development Installation

For local development and testing:

```bash
# Clone repository
git clone https://github.com/solifugus/amorphdb.git
cd amorphdb

# Build and install
make build
make install

# Start AmorphDB
amorphd &
amorph
```

### 2. System Installation (Recommended for Production)

Complete system installation with service management:

```bash
# Clone repository
git clone https://github.com/solifugus/amorphdb.git
cd amorphdb

# System-wide installation (requires sudo)
sudo make install-system

# Start service
sudo systemctl start amorphd
sudo systemctl enable amorphd

# Connect
amorph
```

### 3. Docker Installation

Containerized deployment:

```bash
# Clone repository
git clone https://github.com/solifugus/amorphdb.git
cd amorphdb

# Build and start Docker mesh
make docker-build
make docker-up

# Connect to any node
amorph --host localhost:8080
```

## Installation Methods

### Development Setup

**Best for:** Local development, testing, learning

```bash
# Build from source
make build

# Install to $GOPATH/bin
make install

# Start standalone
amorphd
```

**Features:**
- ✅ Quick setup
- ✅ No system modifications
- ✅ Easy to remove
- ❌ No service management
- ❌ Manual startup required

### System Installation

**Best for:** Production servers, permanent installations

```bash
# Full system installation
sudo make install-system
```

**What it installs:**
- Binaries → `/usr/local/bin/`
- Configuration → `/etc/amorphdb/`
- Data directory → `/var/lib/amorphdb/`
- Logs → `/var/log/amorphdb/`
- Systemd service → `amorphd.service`
- System user → `amorphdb`

**Features:**
- ✅ Automatic startup
- ✅ Service management
- ✅ Proper permissions
- ✅ Log rotation
- ✅ Security hardening

### Docker Deployment

**Best for:** Development, testing, cloud deployments

#### Single Node

```bash
# Build image
make docker-build

# Run single node
docker run -d \
  --name amorphdb \
  -p 8080:8080 \
  -v amorphdb_data:/var/lib/amorphdb \
  amorphdb:latest
```

#### Three-Node Mesh

```bash
# Start complete mesh
make docker-up

# View logs
make docker-logs

# Stop mesh
make docker-down
```

**Features:**
- ✅ Isolated environment
- ✅ Easy scaling
- ✅ Consistent deployment
- ✅ Volume persistence

## Configuration

### Default Configuration

System installation creates `/etc/amorphdb/amorphd.yaml`:

```yaml
data:
  storage_dir: "/var/lib/amorphdb"

network:
  port: 8080
  local_socket_path: "/var/run/amorphdb.sock"
  max_connections: 1000

mesh:
  name: ""          # Standalone mode
  identity: "node1" # Node identifier

logging:
  level: "info"
  file: "/var/log/amorphdb/amorphdb.log"

security:
  enable_auth: false  # Enable for production
```

### Environment Variables

Docker and development installations support:

```bash
AMORPHDB_PORT=8080
AMORPHDB_DATA_DIR=/var/lib/amorphdb
AMORPHDB_LOG_LEVEL=info
AMORPHDB_MESH_NAME=""
```

## Service Management

### Systemd (System Installation)

```bash
# Service control
sudo systemctl start amorphd
sudo systemctl stop amorphd
sudo systemctl restart amorphd
sudo systemctl status amorphd

# Enable auto-start
sudo systemctl enable amorphd

# View logs
sudo journalctl -u amorphd -f
```

### Docker

```bash
# Container management
docker start amorphdb
docker stop amorphdb
docker logs amorphdb

# Docker Compose
docker-compose up -d
docker-compose down
docker-compose logs -f
```

## Creating Your First Mesh

Once AmorphDB is installed:

### 1. Start as Mesh Founder

```bash
# Create named mesh
amorphctl create-mesh "my-company"

# Check status
amorphctl status
```

### 2. Add More Nodes

On additional servers:

```bash
# Join existing mesh
amorphctl join <founder-address>:8080

# Verify mesh membership
amorphctl mesh members
```

### 3. Bridge to Other Meshes

```bash
# Connect to partner mesh
amorphctl bridge partner.example.com:8080

# Access cross-mesh data
amorph
> my.partner.shared.config
```

## Troubleshooting

### Common Issues

**1. Permission Denied**
```bash
# Fix data directory permissions
sudo chown -R amorphdb:amorphdb /var/lib/amorphdb
```

**2. Port Already in Use**
```bash
# Check what's using port 8080
sudo netstat -tlpn | grep 8080

# Use different port
amorphd --port=8081
```

**3. Service Won't Start**
```bash
# Check service status
sudo systemctl status amorphd

# View detailed logs
sudo journalctl -u amorphd -n 50
```

**4. Docker Issues**
```bash
# Check container logs
docker logs amorphdb

# Rebuild image
make docker-build

# Reset volumes
docker-compose down -v
```

### Log Locations

- **System Install:** `/var/log/amorphdb/amorphdb.log`
- **Docker:** `docker logs <container>`
- **Development:** Console output

### Getting Help

- Documentation: `docs/`
- Examples: `examples/`
- Issues: [GitHub Issues](https://github.com/solifugus/amorphdb/issues)

## Uninstallation

### Development Install

```bash
# Remove from $GOPATH/bin
rm $GOPATH/bin/{amorphd,amorph,amorphctl}
```

### System Install

```bash
# Complete removal
sudo make uninstall-system

# Remove data (optional, destructive)
sudo rm -rf /var/lib/amorphdb /etc/amorphdb /var/log/amorphdb
```

### Docker

```bash
# Stop and remove
make docker-down

# Remove image
docker rmi amorphdb:latest
```

## Next Steps

1. **Learn AmorphDB:** Work through `docs/AmorphDB_Tutorial.md`
2. **Mesh Operations:** Read `docs/mesh_management_guide.md`
3. **Examples:** Explore `examples/` directory
4. **Production Setup:** Review security and performance settings

Welcome to AmorphDB! 🚀