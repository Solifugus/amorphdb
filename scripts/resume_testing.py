#!/usr/bin/env python3
"""
Resume Phase 3 Testing - Section 3.2.3: Network and Distributed Performance
Use working SSH deployment method from Sessions 1-3
"""

import subprocess
import time
import tempfile
import os

class ResumePhase3Testing:
    def __init__(self):
        # VM configuration from working sessions
        self.nodes = {
            "ra-do-ki": {"ip": "192.168.122.10", "port": 5000, "hostname": "amorphdb0"},
            "fi-ne-so": {"ip": "192.168.122.11", "port": 5000, "hostname": "amorphdb1"},
            "lu-ma-te": {"ip": "192.168.122.12", "port": 5000, "hostname": "amorphdb2"}
        }

    def ssh_command(self, ip, command, timeout=10):
        """Execute SSH command on remote node"""
        ssh_cmd = [
            "ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5",
            f"amorphdb@{ip}", command
        ]

        try:
            result = subprocess.run(ssh_cmd, capture_output=True, text=True, timeout=timeout)
            return result
        except subprocess.TimeoutExpired:
            print(f"⏰ Command timed out on {ip}")
            return None

    def deploy_and_run_test(self, node_id, test_script_content, script_name):
        """Deploy MBL script and execute on VM"""
        config = self.nodes[node_id]
        ip = config["ip"]
        port = config["port"]

        print(f"🚀 Running {script_name} on {node_id} ({config['hostname']})...")

        # Write script to temporary file
        with tempfile.NamedTemporaryFile(mode='w', suffix='.mbl', delete=False) as f:
            f.write(test_script_content)
            temp_path = f.name

        try:
            # Copy script to VM
            scp_result = subprocess.run([
                "scp", "-o", "StrictHostKeyChecking=no",
                temp_path, f"amorphdb@{ip}:/tmp/{script_name}"
            ], capture_output=True)

            if scp_result.returncode != 0:
                print(f"  ❌ Failed to copy script: {scp_result.stderr.decode()}")
                return False

            # Check if AmorphDB binary exists
            check_result = self.ssh_command(ip, "ls -la /home/amorphdb/amorphdb/bin/amorph")
            if not check_result or check_result.returncode != 0:
                print(f"  ❌ AmorphDB binary not found on {ip}")
                print(f"  Directory check: {check_result.stderr if check_result else 'SSH failed'}")
                return False

            print(f"  ✅ AmorphDB binary confirmed on {ip}")

            # Execute script
            exec_command = f"/home/amorphdb/amorphdb/bin/amorph -node {ip}:{port} -run /tmp/{script_name}"
            exec_result = self.ssh_command(ip, exec_command, timeout=30)

            if exec_result and exec_result.returncode == 0:
                print(f"  ✅ Test executed successfully!")
                print(f"  Output: {exec_result.stdout}")
                return True
            else:
                print(f"  ❌ Test execution failed")
                if exec_result:
                    print(f"  Error: {exec_result.stderr}")
                return False

        finally:
            # Clean up temporary file
            os.unlink(temp_path)

    def test_connectivity_validation(self):
        """Test basic connectivity before proceeding"""
        print("🔍 VALIDATING VM CONNECTIVITY AND AMORPHDB DEPLOYMENT")
        print("=" * 60)

        simple_test = '''output("=== Phase 3 Connectivity Validation ===")
test.connectivity.validation_time = "@2026-03-10 06:45:00"
test.connectivity.node_status = "operational"
test.connectivity.amorphdb_version = "phase3_production_ready"
output("Connectivity validation successful - ready for Section 3.2.3")
'''

        # Test on first available node
        for node_id in ["ra-do-ki", "fi-ne-so", "lu-ma-te"]:
            if self.deploy_and_run_test(node_id, simple_test, "connectivity_test.mbl"):
                print(f"\n✅ CONNECTIVITY VALIDATED ON {node_id.upper()}")
                print("🚀 Ready to proceed with Section 3.2.3 testing!")
                return True
            print(f"❌ Failed on {node_id}, trying next node...")

        print("❌ All nodes failed connectivity test")
        return False

    def show_next_steps(self):
        """Show what to do next"""
        print("\n" + "=" * 60)
        print("📋 NEXT STEPS FOR SECTION 3.2.3:")
        print("  P3.1: Replication overhead and bandwidth usage")
        print("  P3.2: Gossip protocol efficiency testing")
        print("  P3.3: Zone migration performance optimization")
        print("  P3.4: Heartbeat timing optimization")
        print("  P3.5: Connection pooling efficiency")
        print("\nReady to create and execute these tests!")

if __name__ == "__main__":
    tester = ResumePhase3Testing()

    if tester.test_connectivity_validation():
        tester.show_next_steps()
    else:
        print("\n⚠️  Need to deploy AmorphDB binaries to VMs first")