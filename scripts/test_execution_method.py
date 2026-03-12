#!/usr/bin/env python3
"""
Test Execution Method - Replicate the working approach from earlier sessions
"""

import subprocess
import time
import os

class ExecutionMethodTest:
    def __init__(self):
        # VM configuration from working sessions
        self.nodes = {
            "ra-do-ki": {"ip": "192.168.122.10", "port": 5000},
            "fi-ne-so": {"ip": "192.168.122.11", "port": 5000},
            "lu-ma-te": {"ip": "192.168.122.12", "port": 5000}
        }

    def check_connectivity(self):
        """Check VM connectivity like the working sessions"""
        print("🔍 CHECKING VM CONNECTIVITY")
        print("-" * 40)

        for node_id, config in self.nodes.items():
            ip = config["ip"]
            print(f"Testing {node_id} at {ip}...")

            # Test ping connectivity
            ping_result = subprocess.run([
                "ping", "-c", "1", "-W", "2", ip
            ], capture_output=True)

            if ping_result.returncode == 0:
                print(f"  ✅ Ping successful")

                # Test SSH connectivity
                ssh_result = subprocess.run([
                    "ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=3",
                    f"amorphdb@{ip}", "hostname"
                ], capture_output=True, text=True)

                if ssh_result.returncode == 0:
                    print(f"  ✅ SSH successful - hostname: {ssh_result.stdout.strip()}")
                else:
                    print(f"  ❌ SSH failed - {ssh_result.stderr.strip()}")
            else:
                print(f"  ❌ Ping failed")

    def test_mbl_execution(self):
        """Test MBL execution using the working method"""
        print("\n🧪 TESTING MBL EXECUTION")
        print("-" * 40)

        # Create a simple test script
        test_script = '''output("=== Simple Connectivity Test ===")
test.connection.start_time = "@2026-03-10 06:30:00"
test.connection.message = "VM connectivity validation"
test.connection.status = "testing"
output("Test script executed successfully")
'''

        # Write test script locally
        with open("/tmp/simple_test.mbl", "w") as f:
            f.write(test_script)

        # Try to deploy and execute on first available node
        for node_id, config in self.nodes.items():
            ip = config["ip"]
            port = config["port"]

            print(f"Testing execution on {node_id} ({ip})...")

            # Copy script to VM
            scp_result = subprocess.run([
                "scp", "-o", "StrictHostKeyChecking=no",
                "/tmp/simple_test.mbl", f"amorphdb@{ip}:/tmp/"
            ], capture_output=True)

            if scp_result.returncode == 0:
                print(f"  ✅ Script copied to VM")

                # Execute script using the working method
                exec_command = f"/home/amorphdb/amorphdb/bin/amorph -node {ip}:{port} -run /tmp/simple_test.mbl"

                ssh_result = subprocess.run([
                    "ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10",
                    f"amorphdb@{ip}", exec_command
                ], capture_output=True, text=True, timeout=30)

                print(f"  Command: {exec_command}")
                print(f"  Return code: {ssh_result.returncode}")
                if ssh_result.stdout:
                    print(f"  Output: {ssh_result.stdout}")
                if ssh_result.stderr:
                    print(f"  Errors: {ssh_result.stderr}")

                if ssh_result.returncode == 0:
                    print(f"  ✅ MBL execution successful!")
                    break
                else:
                    print(f"  ❌ MBL execution failed")
            else:
                print(f"  ❌ Script copy failed")

    def main(self):
        print("🔧 TESTING EXECUTION METHOD FROM WORKING SESSIONS")
        print("=" * 60)

        self.check_connectivity()
        self.test_mbl_execution()

        print("\n" + "=" * 60)
        print("🔧 EXECUTION METHOD TEST COMPLETE")

if __name__ == "__main__":
    test = ExecutionMethodTest()
    test.main()