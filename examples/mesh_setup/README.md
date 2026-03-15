# Mesh Setup Examples

This directory contains practical examples for setting up AmorphDB meshes in various deployment scenarios.

## Quick Start Examples

### 1. Single Node Development Setup

The simplest possible setup for development:

```bash
# Start a standalone node
./standalone_dev.sh
```

See: [standalone_dev.sh](standalone_dev.sh)

### 2. Three-Node Local Mesh

Create a three-node mesh on a single machine for testing:

```bash
# Set up local test mesh
./three_node_local.sh
```

See: [three_node_local.sh](three_node_local.sh)

### 3. Production Cluster

Deploy a production-ready cluster across multiple servers:

```bash
# Deploy to production servers
./production_cluster.sh
```

See: [production_cluster.sh](production_cluster.sh)

### 4. Docker Compose Setup

Container-based deployment for development and CI:

```bash
# Start containerized mesh
docker-compose -f docker_mesh.yml up
```

See: [docker_mesh.yml](docker_mesh.yml)

## Configuration Examples

- [basic_config.yaml](configs/basic_config.yaml) - Minimal working configuration
- [production_config.yaml](configs/production_config.yaml) - Production-ready configuration
- [multi_mesh_config.yaml](configs/multi_mesh_config.yaml) - Configuration with multiple bridges

## Deployment Scenarios

Each example includes:
- Complete setup scripts
- Configuration files
- Verification steps
- Troubleshooting tips

Choose the example that best matches your deployment scenario and customize as needed.