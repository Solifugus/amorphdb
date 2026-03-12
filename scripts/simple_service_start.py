#!/usr/bin/env python3
"""
Simple AmorphDB Service Starter
Direct approach to start services and run tests
"""

import subprocess
import time

class SimpleServiceManager:
    def __init__(self):
        self.nodes = {
            "ra-do-ki": {"ip": "192.168.122.10", "port": 5000},
            "fi-ne-so": {"ip": "192.168.122.11", "port": 5000},
            "lu-ma-te": {"ip": "192.168.122.12", "port": 5000}
        }

    def ssh_command(self, ip, command):
        """Execute SSH command"""
        ssh_cmd = ["ssh", "-o", "ConnectTimeout=5", f"amorphdb@{ip}", command]
        return subprocess.run(ssh_cmd, capture_output=True, text=True)

    def start_services_and_test(self):
        """Start services with simple approach and run one test"""
        print("🚀 SIMPLE SERVICE START AND TEST")
        print("=" * 50)

        # Start services on all nodes
        for node_id, config in self.nodes.items():
            ip = config["ip"]
            port = config["port"]

            print(f"\n📍 {node_id} ({ip}:{port})")

            # Kill existing processes
            print("🔧 Cleanup...")
            self.ssh_command(ip, "pkill amorphd || true")
            time.sleep(1)

            # Start service (simple background start)
            print("🚀 Starting service...")
            start_cmd = f"cd /home/amorphdb && nohup ./amorphd -node {node_id} -port {port} > /tmp/amorphd_{node_id}.log 2>&1 &"
            result = self.ssh_command(ip, start_cmd)

            print(f"   Start result: {result.returncode}")

            # Brief wait
            time.sleep(2)

            # Check if running
            check_result = self.ssh_command(ip, "ps aux | grep amorphd | grep -v grep")
            if check_result.returncode == 0 and check_result.stdout.strip():
                print("   ✅ Service running")
            else:
                print("   ❌ Service not detected")

        print(f"\n⏳ Waiting for services to stabilize...")
        time.sleep(5)

        # Try a simple test
        print(f"\n🧪 Testing connectivity with simple MBL script...")

        simple_test = '''output("=== Simple Connectivity Test ===")
test.simple.connection = "working"
test.simple.timestamp = "@2026-03-10 07:30:00"
output("Test completed successfully")
'''

        # Write test to local file
        with open("/tmp/simple_test.mbl", "w") as f:
            f.write(simple_test)

        # Try test on first node
        node_id = "ra-do-ki"
        ip = self.nodes[node_id]["ip"]
        port = self.nodes[node_id]["port"]

        print(f"📤 Deploying test to {node_id}...")

        # Copy test file
        scp_result = subprocess.run([
            "scp", "-o", "StrictHostKeyChecking=no",
            "/tmp/simple_test.mbl", f"amorphdb@{ip}:/tmp/"
        ], capture_output=True)

        if scp_result.returncode == 0:
            print("✅ Test deployed")

            # Execute test
            print("🎯 Executing test...")
            exec_cmd = f"cd /home/amorphdb && echo 'output(\"Testing direct execution\")' | ./amorph"

            exec_result = self.ssh_command(ip, exec_cmd)

            print(f"Execution result: {exec_result.returncode}")
            if exec_result.stdout:
                print(f"Output: {exec_result.stdout}")
            if exec_result.stderr:
                print(f"Errors: {exec_result.stderr}")

            # Try alternative execution
            print("\n🎯 Trying script file execution...")
            script_cmd = f"cd /home/amorphdb && ./amorph < /tmp/simple_test.mbl"

            script_result = self.ssh_command(ip, script_cmd)

            print(f"Script result: {script_result.returncode}")
            if script_result.stdout:
                print(f"Output: {script_result.stdout}")
            if script_result.stderr:
                print(f"Errors: {script_result.stderr}")

        else:
            print("❌ Failed to deploy test")

if __name__ == "__main__":
    manager = SimpleServiceManager()
    manager.start_services_and_test()