#!/usr/bin/env python3
"""
Diagnose AmorphDB Deployment on VMs
Check what's actually deployed and working
"""

import subprocess

class AmorphDBDiagnostics:
    def __init__(self):
        self.nodes = {
            "ra-do-ki": {"ip": "192.168.122.10", "hostname": "amorphdb0"},
            "fi-ne-so": {"ip": "192.168.122.11", "hostname": "amorphdb1"},
            "lu-ma-te": {"ip": "192.168.122.12", "hostname": "amorphdb2"}
        }

    def ssh_command(self, ip, command, timeout=10):
        """Execute SSH command with diagnostics"""
        ssh_cmd = [
            "ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5",
            f"amorphdb@{ip}", command
        ]

        try:
            result = subprocess.run(ssh_cmd, capture_output=True, text=True, timeout=timeout)
            return result
        except subprocess.TimeoutExpired:
            return None

    def diagnose_node(self, node_id, config):
        """Comprehensive node diagnosis"""
        ip = config["ip"]
        hostname = config["hostname"]

        print(f"\n🔍 DIAGNOSING {node_id} ({hostname} - {ip})")
        print("=" * 50)

        # Check basic connectivity
        print("1. SSH Connectivity Test:")
        result = self.ssh_command(ip, "echo 'SSH OK'", timeout=5)
        if result and result.returncode == 0:
            print("   ✅ SSH working")
        else:
            print("   ❌ SSH failed")
            return

        # Check home directory structure
        print("\n2. Home Directory Structure:")
        result = self.ssh_command(ip, "ls -la /home/amorphdb/", timeout=5)
        if result and result.returncode == 0:
            print("   📁 /home/amorphdb/:")
            for line in result.stdout.strip().split('\n')[:10]:  # First 10 lines
                print(f"      {line}")
        else:
            print("   ❌ Cannot list home directory")

        # Check for AmorphDB directory
        print("\n3. AmorphDB Installation Check:")
        result = self.ssh_command(ip, "ls -la /home/amorphdb/amorphdb/", timeout=5)
        if result and result.returncode == 0:
            print("   ✅ AmorphDB directory exists")
            print("   📁 Contents:")
            for line in result.stdout.strip().split('\n')[:10]:
                print(f"      {line}")
        else:
            print("   ❌ No AmorphDB directory found")
            print("   💡 Need to deploy AmorphDB first")

        # Check for binaries
        print("\n4. Binary Check:")
        for binary in ["amorphd", "amorph", "amorphctl"]:
            result = self.ssh_command(ip, f"ls -la /home/amorphdb/amorphdb/bin/{binary}", timeout=5)
            if result and result.returncode == 0:
                size = result.stdout.split()[4] if result.stdout else "unknown"
                print(f"   ✅ {binary} ({size} bytes)")
            else:
                print(f"   ❌ {binary} missing")

        # Check if binaries are executable
        print("\n5. Binary Permissions:")
        result = self.ssh_command(ip, "ls -la /home/amorphdb/amorphdb/bin/", timeout=5)
        if result and result.returncode == 0:
            for line in result.stdout.strip().split('\n')[1:]:  # Skip total line
                if 'amorph' in line:
                    permissions = line.split()[0]
                    filename = line.split()[-1]
                    executable = 'x' in permissions
                    status = "✅" if executable else "❌"
                    print(f"   {status} {filename}: {permissions}")

        # Test amorph binary execution
        print("\n6. Binary Execution Test:")
        result = self.ssh_command(ip, "cd /home/amorphdb/amorphdb && ./bin/amorph --help 2>&1 | head -5", timeout=5)
        if result:
            if result.returncode == 0 and result.stdout.strip():
                print("   ✅ amorph binary executes")
                print(f"   📄 Output: {result.stdout.strip()[:100]}...")
            else:
                print("   ❌ amorph execution failed")
                print(f"   🔧 Error: {result.stderr.strip()[:100]}")
        else:
            print("   ⏰ Binary execution timed out")

        # Check data directory
        print("\n7. Data Directory Check:")
        result = self.ssh_command(ip, f"ls -la /home/amorphdb/data/", timeout=5)
        if result and result.returncode == 0:
            print("   ✅ Data directory exists")
        else:
            print("   ❌ Data directory missing")
            print("   💡 Creating data directory...")
            create_result = self.ssh_command(ip, f"mkdir -p /home/amorphdb/data/{node_id}", timeout=5)
            if create_result and create_result.returncode == 0:
                print("   ✅ Data directory created")
            else:
                print("   ❌ Failed to create data directory")

    def run_diagnostics(self):
        """Run full diagnostics on all nodes"""
        print("🏥 AMORPHDB DEPLOYMENT DIAGNOSTICS")
        print("=" * 60)

        for node_id, config in self.nodes.items():
            self.diagnose_node(node_id, config)

        print(f"\n" + "=" * 60)
        print("🏁 DIAGNOSTICS COMPLETE")
        print("\n💡 NEXT STEPS:")
        print("1. If binaries missing: Deploy AmorphDB to VMs")
        print("2. If permissions wrong: Fix executable permissions")
        print("3. If directories missing: Create required directories")
        print("4. Re-run service startup after fixing issues")

if __name__ == "__main__":
    diagnostics = AmorphDBDiagnostics()
    diagnostics.run_diagnostics()