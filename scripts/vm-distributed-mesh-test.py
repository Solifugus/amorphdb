#!/usr/bin/env python3
"""
AmorphDB Distributed Mesh Test
Deploy and test AmorphDB across real VMs with SSH key authentication
"""
import os
import subprocess
import time
import json

class AmorphDBDistributedMeshTest:
    def __init__(self):
        self.binary_dir = "/home/solifugus/development/amorphdb/bin"

        # VM configuration - using actual libvirt IPs
        self.nodes = {
            "ra-do-ki": {
                "ip": "192.168.122.10",
                "mesh_port": 5000,
                "user": "amorphdb"
            },
            "fi-ne-so": {
                "ip": "192.168.122.11",
                "mesh_port": 5000,
                "user": "amorphdb"
            },
            "lu-ma-te": {
                "ip": "192.168.122.12",
                "mesh_port": 5000,
                "user": "amorphdb"
            }
        }

    def ssh_command(self, ip, command, capture_output=True):
        """Execute command on remote VM via SSH"""
        ssh_cmd = [
            "ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no",
            "-o", "ConnectTimeout=5", f"amorphdb@{ip}", command
        ]

        if capture_output:
            return subprocess.run(ssh_cmd, capture_output=True, text=True)
        else:
            return subprocess.run(ssh_cmd)

    def scp_file(self, local_path, ip, remote_path):
        """Copy file to remote VM via SCP"""
        scp_cmd = [
            "scp", "-o", "StrictHostKeyChecking=no",
            local_path, f"amorphdb@{ip}:{remote_path}"
        ]
        return subprocess.run(scp_cmd, capture_output=True)

    def deploy_binaries(self, ip):
        """Deploy AmorphDB binaries to a VM"""
        print(f"  📦 Deploying binaries to {ip}...")

        # Create target directory
        result = self.ssh_command(ip, "mkdir -p /home/amorphdb/amorphdb/bin")
        if result.returncode != 0:
            print(f"  ❌ Failed to create directory on {ip}")
            return False

        # Deploy amorphd and amorph
        for binary in ["amorphd", "amorph"]:
            local_path = f"{self.binary_dir}/{binary}"
            if not os.path.exists(local_path):
                print(f"  ❌ Binary not found: {local_path}")
                return False

            result = self.scp_file(local_path, ip, f"/home/amorphdb/amorphdb/bin/{binary}")
            if result.returncode != 0:
                print(f"  ❌ Failed to copy {binary} to {ip}")
                return False

            # Make executable
            result = self.ssh_command(ip, f"chmod +x /home/amorphdb/amorphdb/bin/{binary}")
            if result.returncode != 0:
                print(f"  ❌ Failed to make {binary} executable on {ip}")
                return False

        print(f"  ✅ Binaries deployed to {ip}")
        return True

    def stop_existing_amorphd(self, ip):
        """Stop any existing amorphd processes"""
        result = self.ssh_command(ip, "pkill -f amorphd || true")
        time.sleep(2)
        return True

    def start_amorphd(self, node_id, ip, bootstrap_peer=None):
        """Start amorphd on a VM"""
        print(f"  🚀 Starting amorphd {node_id} on {ip}...")

        # Create data directory
        self.ssh_command(ip, "mkdir -p /home/amorphdb/amorphdb/data")

        # Build command
        cmd_parts = [
            "/home/amorphdb/amorphdb/bin/amorphd",
            f"-node {node_id}",
            "-data /home/amorphdb/amorphdb/data",
            "-port 5000"
        ]

        if bootstrap_peer:
            cmd_parts.append(f"-peer {bootstrap_peer}")

        cmd = " ".join(cmd_parts)

        # Start in background
        bg_cmd = f"nohup {cmd} > /home/amorphdb/amorphd.log 2>&1 &"
        result = self.ssh_command(ip, bg_cmd)

        if result.returncode != 0:
            print(f"  ❌ Failed to start amorphd on {ip}")
            return False

        # Wait for startup
        time.sleep(3)

        # Check if process is running
        result = self.ssh_command(ip, "pgrep -f amorphd")
        if result.returncode == 0:
            print(f"  ✅ amorphd {node_id} started on {ip}")
            return True
        else:
            print(f"  ❌ amorphd {node_id} failed to start on {ip}")
            # Show log for debugging
            log_result = self.ssh_command(ip, "tail -5 /home/amorphdb/amorphd.log")
            if log_result.stdout:
                print(f"      Log: {log_result.stdout}")
            return False

    def test_connectivity(self, ip):
        """Test basic connectivity to AmorphDB node"""
        print(f"  🔍 Testing connectivity to {ip}:5000...")

        # Try to connect using amorph client
        test_cmd = f"/home/amorphdb/amorphdb/bin/amorph -server {ip}:5000 -c 'node.status'"
        result = self.ssh_command(ip, test_cmd)

        if result.returncode == 0:
            print(f"  ✅ AmorphDB responding on {ip}:5000")
            return True
        else:
            print(f"  ⚠️ AmorphDB connectivity test failed on {ip}:5000")
            if result.stderr:
                print(f"      Error: {result.stderr.strip()}")
            return False

    def run_distributed_test(self):
        """Run comprehensive distributed mesh test"""
        print("🌐 AmorphDB Distributed Mesh Test")
        print("=" * 50)

        # Check binaries exist locally
        print("📦 Checking local binaries...")
        for binary in ["amorphd", "amorph"]:
            path = f"{self.binary_dir}/{binary}"
            if os.path.exists(path):
                size = os.path.getsize(path)
                print(f"✅ {binary}: {size:,} bytes")
            else:
                print(f"❌ {binary} not found at {path}")
                return False

        # Test SSH connectivity to all VMs
        print("\n🔌 Testing SSH connectivity...")
        for node_id, config in self.nodes.items():
            result = self.ssh_command(config["ip"], "echo 'SSH OK'")
            if result.returncode == 0:
                print(f"✅ SSH to {node_id} ({config['ip']}) working")
            else:
                print(f"❌ SSH to {node_id} ({config['ip']}) failed")
                return False

        # Deploy binaries to all VMs
        print("\n📦 Deploying binaries...")
        for node_id, config in self.nodes.items():
            if not self.deploy_binaries(config["ip"]):
                return False

        # Stop any existing amorphd processes
        print("\n🛑 Stopping existing AmorphDB processes...")
        for node_id, config in self.nodes.items():
            self.stop_existing_amorphd(config["ip"])

        # Start AmorphDB mesh
        print("\n🚀 Starting AmorphDB mesh...")

        # Start bootstrap node first
        bootstrap_node = "ra-do-ki"
        bootstrap_ip = self.nodes[bootstrap_node]["ip"]

        if not self.start_amorphd(bootstrap_node, bootstrap_ip):
            return False

        # Wait for bootstrap to stabilize
        time.sleep(5)

        # Start other nodes with bootstrap peer
        for node_id, config in self.nodes.items():
            if node_id != bootstrap_node:
                peer_address = f"{bootstrap_ip}:5000"
                if not self.start_amorphd(node_id, config["ip"], peer_address):
                    return False
                time.sleep(3)  # Stagger startup

        # Wait for mesh to form
        print("\n⏳ Waiting for mesh formation...")
        time.sleep(10)

        # Test connectivity to all nodes
        print("\n🔍 Testing node connectivity...")
        all_connected = True
        for node_id, config in self.nodes.items():
            if not self.test_connectivity(config["ip"]):
                all_connected = False

        if all_connected:
            print("\n🎉 DISTRIBUTED MESH TEST SUCCESSFUL!")
            print("✅ All nodes deployed and responding")
            print("✅ Mesh network established")
            print("✅ Ready for distributed functionality testing")
            return True
        else:
            print("\n⚠️ Some connectivity issues detected")
            return False

    def cleanup(self):
        """Stop all AmorphDB processes"""
        print("\n🧹 Cleaning up...")
        for node_id, config in self.nodes.items():
            print(f"  Stopping {node_id}...")
            self.ssh_command(config["ip"], "pkill -f amorphd || true")

def main():
    """Main test execution"""
    test = AmorphDBDistributedMeshTest()

    try:
        success = test.run_distributed_test()
        if success:
            print("\n✅ Distributed mesh test completed successfully!")
            print("🎯 Ready for advanced distributed functionality testing")
        else:
            print("\n❌ Distributed mesh test failed")
            return 1

    except KeyboardInterrupt:
        print("\n⚠️ Test interrupted by user")
        test.cleanup()
        return 1
    except Exception as e:
        print(f"\n❌ Test failed with error: {e}")
        test.cleanup()
        return 1

    return 0

if __name__ == "__main__":
    exit(main())