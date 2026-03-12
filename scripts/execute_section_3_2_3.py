#!/usr/bin/env python3
"""
Execute Section 3.2.3: Network and Distributed Performance Testing
Deploy and run P3.1-P3.5 tests on 3-node VM cluster
"""

import subprocess
import time
import tempfile
import os
from datetime import datetime

class Section32NetworkTesting:
    def __init__(self):
        # VM configuration from working sessions
        self.nodes = {
            "ra-do-ki": {"ip": "192.168.122.10", "port": 5000, "hostname": "amorphdb0"},
            "fi-ne-so": {"ip": "192.168.122.11", "port": 5000, "hostname": "amorphdb1"},
            "lu-ma-te": {"ip": "192.168.122.12", "port": 5000, "hostname": "amorphdb2"}
        }

        # Test files to execute
        self.tests = [
            {"file": "test_p3_1_replication_overhead.mbl", "name": "P3.1 Replication Overhead", "node": "ra-do-ki"},
            {"file": "test_p3_2_gossip_protocol.mbl", "name": "P3.2 Gossip Protocol", "node": "fi-ne-so"},
            {"file": "test_p3_3_zone_migration.mbl", "name": "P3.3 Zone Migration", "node": "lu-ma-te"},
            {"file": "test_p3_4_heartbeat_optimization.mbl", "name": "P3.4 Heartbeat Optimization", "node": "ra-do-ki"},
            {"file": "test_p3_5_connection_pooling.mbl", "name": "P3.5 Connection Pooling", "node": "fi-ne-so"}
        ]

        self.results = []
        self.start_time = datetime.now()

    def ssh_command(self, ip, command, timeout=30):
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

    def deploy_and_execute_test(self, test_info):
        """Deploy and execute a single test"""
        node_id = test_info["node"]
        test_file = test_info["file"]
        test_name = test_info["name"]

        config = self.nodes[node_id]
        ip = config["ip"]
        port = config["port"]
        hostname = config["hostname"]

        print(f"\n🧪 EXECUTING {test_name}")
        print(f"📍 Target: {node_id} ({hostname} - {ip})")
        print("-" * 60)

        # Check if test file exists
        if not os.path.exists(test_file):
            print(f"❌ Test file {test_file} not found!")
            return False

        try:
            # Copy script to VM
            print("📤 Deploying test script...")
            scp_result = subprocess.run([
                "scp", "-o", "StrictHostKeyChecking=no",
                test_file, f"amorphdb@{ip}:/tmp/"
            ], capture_output=True, timeout=30)

            if scp_result.returncode != 0:
                print(f"❌ Failed to deploy script: {scp_result.stderr.decode()}")
                return False

            print("✅ Test script deployed successfully")

            # Execute test
            print("🚀 Executing test script...")
            exec_command = f"/home/amorphdb/amorphdb/bin/amorph -node {ip}:{port} -run /tmp/{test_file}"

            exec_result = self.ssh_command(ip, exec_command, timeout=60)

            if exec_result and exec_result.returncode == 0:
                print("✅ TEST PASSED")
                print(f"📊 Output preview: {exec_result.stdout[:200]}...")

                # Store result
                self.results.append({
                    "test": test_name,
                    "status": "PASSED",
                    "node": f"{node_id} ({hostname})",
                    "execution_time": "< 60s",
                    "output_preview": exec_result.stdout[:500]
                })
                return True
            else:
                print("❌ TEST FAILED")
                if exec_result:
                    print(f"Error: {exec_result.stderr}")
                    self.results.append({
                        "test": test_name,
                        "status": "FAILED",
                        "node": f"{node_id} ({hostname})",
                        "error": exec_result.stderr[:300]
                    })
                return False

        except subprocess.TimeoutExpired:
            print("⏰ Test execution timed out")
            self.results.append({
                "test": test_name,
                "status": "TIMEOUT",
                "node": f"{node_id} ({hostname})"
            })
            return False
        except Exception as e:
            print(f"💥 Unexpected error: {e}")
            self.results.append({
                "test": test_name,
                "status": "ERROR",
                "node": f"{node_id} ({hostname})",
                "error": str(e)
            })
            return False

    def validate_connectivity(self):
        """Validate VM connectivity before testing"""
        print("🔍 VALIDATING VM CONNECTIVITY")
        print("=" * 60)

        all_connected = True
        for node_id, config in self.nodes.items():
            ip = config["ip"]
            hostname = config["hostname"]

            print(f"Testing {node_id} ({hostname}) at {ip}...")

            # Test SSH connectivity
            result = self.ssh_command(ip, "echo 'Connected successfully'", timeout=5)

            if result and result.returncode == 0:
                print(f"  ✅ SSH connectivity confirmed")

                # Check AmorphDB binary
                binary_check = self.ssh_command(ip, "ls -la /home/amorphdb/amorphdb/bin/amorph", timeout=5)
                if binary_check and binary_check.returncode == 0:
                    print(f"  ✅ AmorphDB binary confirmed")
                else:
                    print(f"  ❌ AmorphDB binary not found!")
                    all_connected = False
            else:
                print(f"  ❌ SSH connectivity failed!")
                all_connected = False

        return all_connected

    def execute_all_tests(self):
        """Execute all Section 3.2.3 tests"""
        print("🚀 SECTION 3.2.3: NETWORK AND DISTRIBUTED PERFORMANCE TESTING")
        print("=" * 70)
        print(f"Start time: {self.start_time.strftime('%Y-%m-%d %H:%M:%S')}")

        # Validate connectivity first
        if not self.validate_connectivity():
            print("\n❌ CONNECTIVITY VALIDATION FAILED")
            print("Cannot proceed with testing - please check VM status")
            return False

        print(f"\n🎯 EXECUTING {len(self.tests)} NETWORK PERFORMANCE TESTS")

        passed_tests = 0
        for i, test_info in enumerate(self.tests, 1):
            print(f"\n📋 TEST {i}/{len(self.tests)}")

            if self.deploy_and_execute_test(test_info):
                passed_tests += 1

            # Small delay between tests
            if i < len(self.tests):
                print("⏸️  Brief pause between tests...")
                time.sleep(5)

        # Generate final report
        self.generate_final_report(passed_tests)

        return passed_tests == len(self.tests)

    def generate_final_report(self, passed_tests):
        """Generate final test report"""
        end_time = datetime.now()
        duration = end_time - self.start_time

        print("\n" + "=" * 70)
        print("📋 SECTION 3.2.3 FINAL REPORT")
        print("=" * 70)
        print(f"Execution time: {duration.total_seconds():.1f} seconds")
        print(f"Tests passed: {passed_tests}/{len(self.tests)}")
        print(f"Success rate: {(passed_tests/len(self.tests)*100):.1f}%")

        print(f"\n📊 DETAILED RESULTS:")
        for result in self.results:
            status_emoji = "✅" if result["status"] == "PASSED" else "❌"
            print(f"  {status_emoji} {result['test']} - {result['status']} on {result['node']}")

        if passed_tests == len(self.tests):
            print(f"\n🎉 SECTION 3.2.3 COMPLETE!")
            print("✅ All network and distributed performance tests passed")
            print("📈 Progress: 35/45 Phase 3 tests complete (77.8%)")
            print("🎯 Ready for Section 3.3: End-to-End Security Testing")
        else:
            print(f"\n⚠️  SECTION 3.2.3 INCOMPLETE")
            print(f"❌ {len(self.tests) - passed_tests} tests failed or timed out")
            print("🔧 Review failed tests and retry")

if __name__ == "__main__":
    tester = Section32NetworkTesting()
    success = tester.execute_all_tests()

    if success:
        print(f"\n🏆 NETWORK PERFORMANCE TESTING SUCCESSFUL!")
        exit(0)
    else:
        print(f"\n💥 NETWORK PERFORMANCE TESTING INCOMPLETE")
        exit(1)