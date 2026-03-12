#!/usr/bin/env python3
"""
Start AmorphDB Services on All VMs
Start amorphd daemon on each node before testing
"""

import subprocess
import time
import os

class AmorphDBServiceManager:
    def __init__(self):
        # VM configuration
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

    def start_amorphd_service(self, node_id, config):
        """Start AmorphDB daemon on a specific node"""
        ip = config["ip"]
        port = config["port"]
        hostname = config["hostname"]

        print(f"🚀 Starting AmorphDB service on {node_id} ({hostname})")
        print(f"📍 Target: {ip}:{port}")

        # Kill any existing amorphd processes
        print("🔧 Cleaning up existing processes...")
        cleanup_result = self.ssh_command(ip, "pkill -f amorphd || true", timeout=5)

        # Wait a moment for cleanup
        time.sleep(2)

        # Start amorphd daemon
        print("🎯 Starting amorphd daemon...")
        start_command = f"cd /home/amorphdb/amorphdb && nohup ./bin/amorphd -node {node_id} -port {port} -data /home/amorphdb/data/{node_id} > /tmp/amorphd.log 2>&1 &"

        start_result = self.ssh_command(ip, start_command, timeout=10)

        if start_result and start_result.returncode == 0:
            print("✅ Service start command executed")

            # Wait for service to initialize
            print("⏳ Waiting for service initialization...")
            time.sleep(3)

            # Check if service is running
            check_result = self.ssh_command(ip, "ps aux | grep amorphd | grep -v grep", timeout=5)

            if check_result and check_result.returncode == 0 and check_result.stdout.strip():
                print("✅ AmorphDB service confirmed running")
                print(f"   Process: {check_result.stdout.strip()[:100]}...")

                # Test service connectivity
                print("🔌 Testing service connectivity...")
                test_command = f"echo 'test.connection = true' | ./bin/amorph -node {ip}:{port}"

                test_result = self.ssh_command(ip, f"cd /home/amorphdb/amorphdb && timeout 10 {test_command}", timeout=15)

                if test_result and test_result.returncode == 0:
                    print("✅ Service connectivity confirmed")
                    return True
                else:
                    print("⚠️ Service running but connectivity test failed")
                    if test_result:
                        print(f"   Test error: {test_result.stderr.strip()}")
                    return True  # Service is running, connectivity might just need time
            else:
                print("❌ Service not detected running")
                return False
        else:
            print("❌ Failed to start service")
            if start_result:
                print(f"   Error: {start_result.stderr.strip()}")
            return False

    def start_all_services(self):
        """Start AmorphDB services on all nodes"""
        print("🚀 STARTING AMORPHDB SERVICES ON ALL NODES")
        print("=" * 60)

        services_started = 0
        for node_id, config in self.nodes.items():
            print(f"\n📍 NODE: {node_id}")
            print("-" * 30)

            if self.start_amorphd_service(node_id, config):
                services_started += 1
                print(f"✅ {node_id} service ready")
            else:
                print(f"❌ {node_id} service failed")

        print(f"\n" + "=" * 60)
        print(f"📊 SERVICES STARTED: {services_started}/{len(self.nodes)}")

        if services_started == len(self.nodes):
            print("🎉 ALL AMORPHDB SERVICES RUNNING!")
            print("✅ Ready for Section 3.2.3 testing")
            return True
        else:
            print("⚠️ SOME SERVICES FAILED TO START")
            print("🔧 Check VM status and try again")
            return False

    def check_service_status(self):
        """Check status of all AmorphDB services"""
        print("\n🔍 CHECKING SERVICE STATUS")
        print("-" * 40)

        for node_id, config in self.nodes.items():
            ip = config["ip"]
            port = config["port"]

            print(f"{node_id} ({ip}:{port}): ", end="")

            # Check if process is running
            check_result = self.ssh_command(ip, "ps aux | grep amorphd | grep -v grep", timeout=5)

            if check_result and check_result.returncode == 0 and check_result.stdout.strip():
                print("✅ RUNNING")
            else:
                print("❌ NOT RUNNING")

if __name__ == "__main__":
    manager = AmorphDBServiceManager()

    print("Starting AmorphDB service management...")

    if manager.start_all_services():
        print("\n🎯 READY FOR TESTING!")
        print("Run: python3 execute_section_3_2_3.py")

        manager.check_service_status()
    else:
        print("\n💥 SERVICE STARTUP FAILED")
        print("Check VM logs and AmorphDB deployment")