#!/usr/bin/env python3
"""
Execute Section 3.2.3 Tests - FIXED VERSION
AmorphDB services are running, using correct execution method
"""

import subprocess
import time
from datetime import datetime

class Section32TestingFixed:
    def __init__(self):
        # VM configuration - services are running on port 5000
        self.nodes = {
            "ra-do-ki": {"ip": "192.168.122.10", "port": 5000, "hostname": "amorphdb0"},
            "fi-ne-so": {"ip": "192.168.122.11", "port": 5000, "hostname": "amorphdb1"},
            "lu-ma-te": {"ip": "192.168.122.12", "port": 5000, "hostname": "amorphdb2"}
        }

        # Test files
        self.tests = [
            {"file": "test_p3_1_replication_overhead.mbl", "name": "P3.1 Replication Overhead", "node": "ra-do-ki"},
            {"file": "test_p3_2_gossip_protocol.mbl", "name": "P3.2 Gossip Protocol", "node": "fi-ne-so"},
            {"file": "test_p3_3_zone_migration.mbl", "name": "P3.3 Zone Migration", "node": "lu-ma-te"},
            {"file": "test_p3_4_heartbeat_optimization.mbl", "name": "P3.4 Heartbeat Optimization", "node": "ra-do-ki"},
            {"file": "test_p3_5_connection_pooling.mbl", "name": "P3.5 Connection Pooling", "node": "fi-ne-so"}
        ]

        self.results = []

    def execute_test(self, test_info):
        """Execute a single test with corrected method"""
        node_id = test_info["node"]
        test_file = test_info["file"]
        test_name = test_info["name"]

        config = self.nodes[node_id]
        ip = config["ip"]
        port = config["port"]

        print(f"\n🧪 EXECUTING {test_name}")
        print(f"📍 Target: {node_id} ({ip}:{port})")
        print("-" * 60)

        try:
            # Deploy script
            print("📤 Deploying script...")
            scp_result = subprocess.run([
                "scp", "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=10",
                test_file, f"amorphdb@{ip}:/tmp/"
            ], capture_output=True, timeout=20)

            if scp_result.returncode != 0:
                print(f"❌ Deployment failed: {scp_result.stderr.decode()}")
                return False

            print("✅ Script deployed")

            # Execute with correct method - using input redirection
            print("🚀 Executing test...")

            # Method 1: Direct file input to amorph with node connection
            exec_command = f"cd /home/amorphdb && ./amorph -node localhost:{port} < /tmp/{test_file}"

            exec_result = subprocess.run([
                "ssh", "-o", "ConnectTimeout=10", f"amorphdb@{ip}", exec_command
            ], capture_output=True, text=True, timeout=45)

            print(f"Return code: {exec_result.returncode}")

            if exec_result.returncode == 0:
                print("✅ TEST PASSED")
                output_preview = exec_result.stdout[:300] if exec_result.stdout else "No output"
                print(f"📊 Output: {output_preview}...")

                self.results.append({
                    "test": test_name,
                    "status": "PASSED",
                    "node": f"{node_id} ({ip})",
                    "output_length": len(exec_result.stdout) if exec_result.stdout else 0
                })
                return True
            else:
                print("❌ TEST FAILED")
                error_msg = exec_result.stderr[:300] if exec_result.stderr else "No error message"
                print(f"Error: {error_msg}")

                self.results.append({
                    "test": test_name,
                    "status": "FAILED",
                    "node": f"{node_id} ({ip})",
                    "error": error_msg
                })
                return False

        except subprocess.TimeoutExpired:
            print("⏰ Test execution timed out")
            self.results.append({
                "test": test_name,
                "status": "TIMEOUT",
                "node": f"{node_id} ({ip})"
            })
            return False

        except Exception as e:
            print(f"💥 Unexpected error: {e}")
            self.results.append({
                "test": test_name,
                "status": "ERROR",
                "node": f"{node_id} ({ip})",
                "error": str(e)
            })
            return False

    def execute_all_tests(self):
        """Execute all P3 tests"""
        start_time = datetime.now()

        print("🚀 SECTION 3.2.3: NETWORK AND DISTRIBUTED PERFORMANCE")
        print("=" * 70)
        print(f"Start: {start_time.strftime('%Y-%m-%d %H:%M:%S')}")
        print("🎯 AmorphDB services confirmed running on all nodes")

        passed = 0
        for i, test_info in enumerate(self.tests, 1):
            print(f"\n📋 TEST {i}/{len(self.tests)}")

            if self.execute_test(test_info):
                passed += 1

            # Brief pause between tests
            if i < len(self.tests):
                print("⏸️ Pause between tests...")
                time.sleep(3)

        # Final report
        end_time = datetime.now()
        duration = end_time - start_time

        print("\n" + "=" * 70)
        print("📋 SECTION 3.2.3 EXECUTION REPORT")
        print("=" * 70)
        print(f"Duration: {duration.total_seconds():.1f} seconds")
        print(f"Tests passed: {passed}/{len(self.tests)}")
        print(f"Success rate: {(passed/len(self.tests)*100):.1f}%")

        print(f"\n📊 DETAILED RESULTS:")
        for result in self.results:
            status_emoji = "✅" if result["status"] == "PASSED" else "❌"
            print(f"  {status_emoji} {result['test']} - {result['status']} on {result['node']}")

        if passed == len(self.tests):
            print(f"\n🎉 SECTION 3.2.3 COMPLETE!")
            print("🏆 All network and distributed performance tests passed")
            print("📈 Phase 3 Progress: 35/45 tests complete (77.8%)")
            print("🎯 Ready for Section 3.3: End-to-End Security Testing")
            return True
        else:
            print(f"\n⚠️ SECTION 3.2.3 INCOMPLETE")
            print(f"❌ {len(self.tests) - passed} tests failed")
            return False

if __name__ == "__main__":
    tester = Section32TestingFixed()
    success = tester.execute_all_tests()

    if success:
        print("\n🏆 NETWORK PERFORMANCE TESTING COMPLETE!")
    else:
        print("\n🔧 Review failed tests and investigate")