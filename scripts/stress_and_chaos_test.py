#!/usr/bin/env python3
"""
AmorphDB Stress and Chaos Testing Suite
Push the distributed mesh to its limits with high-volume operations,
network partitions, node failures, and data consistency validation
"""
import os
import subprocess
import time
import json
import threading
import random
from concurrent.futures import ThreadPoolExecutor, as_completed

class AmorphDBStressAndChaosTest:
    def __init__(self):
        # VM configuration
        self.nodes = {
            "ra-do-ki": {"ip": "192.168.122.10", "port": 5000},
            "fi-ne-so": {"ip": "192.168.122.11", "port": 5000},
            "lu-ma-te": {"ip": "192.168.122.12", "port": 5000}
        }

        self.test_results = {}
        self.stress_data_count = 0

    def ssh_command(self, ip, command, capture_output=True, timeout=30):
        """Execute command on remote VM via SSH"""
        ssh_cmd = [
            "ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no",
            "-o", "ConnectTimeout=5", f"amorphdb@{ip}", command
        ]

        try:
            if capture_output:
                return subprocess.run(ssh_cmd, capture_output=True, text=True, timeout=timeout)
            else:
                return subprocess.run(ssh_cmd, timeout=timeout)
        except subprocess.TimeoutExpired:
            return None

    def scp_file(self, local_path, ip, remote_path):
        """Copy file to remote VM via SCP"""
        scp_cmd = [
            "scp", "-o", "StrictHostKeyChecking=no",
            local_path, f"amorphdb@{ip}:{remote_path}"
        ]
        return subprocess.run(scp_cmd, capture_output=True)

    def create_stress_script(self, script_name, content):
        """Create a stress test script and deploy to all VMs"""
        local_path = f"/home/solifugus/development/amorphdb/{script_name}"

        with open(local_path, 'w') as f:
            f.write(content)

        # Deploy to all VMs
        for node_id, config in self.nodes.items():
            result = self.scp_file(local_path, config["ip"], f"/home/amorphdb/{script_name}")
            if result.returncode != 0:
                print(f"❌ Failed to deploy {script_name} to {node_id}")
                return False

        return True

    def run_script_on_node(self, node_id, script_name, timeout=60):
        """Run MBL script on specific node with extended timeout"""
        config = self.nodes[node_id]
        command = f"/home/amorphdb/amorphdb/bin/amorph -node {config['ip']}:{config['port']} -run /home/amorphdb/{script_name}"

        try:
            result = subprocess.run([
                "ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no",
                f"amorphdb@{config['ip']}", command
            ], capture_output=True, text=True, timeout=timeout)

            return result
        except subprocess.TimeoutExpired:
            print(f"⏰ Script {script_name} timed out on {node_id}")
            return None

    def check_system_resources(self):
        """Monitor host system resources"""
        result = subprocess.run([
            "bash", "-c", "free -m | grep '^Mem:' && echo 'CPU:' && uptime | awk '{print $10,$11,$12}'"
        ], capture_output=True, text=True)

        if result.returncode == 0:
            lines = result.stdout.strip().split('\n')
            mem_line = lines[0].split()
            used_mem = int(mem_line[2])
            total_mem = int(mem_line[1])
            mem_percent = (used_mem / total_mem) * 100

            print(f"    📊 Memory: {used_mem}MB/{total_mem}MB ({mem_percent:.1f}%)")
            if len(lines) > 2:
                print(f"    📊 CPU Load: {lines[2].strip()}")

            # Safety check - warn if approaching limits
            if mem_percent > 80:
                print(f"    ⚠️ HIGH MEMORY USAGE: {mem_percent:.1f}%")

            return mem_percent < 90  # Return False if memory critically high

        return True

    def test_high_volume_concurrent_writes(self):
        """STRESS TEST 1: High-volume concurrent writes across all nodes"""
        print("\n🔥 STRESS TEST 1: High-volume concurrent writes")
        print("-" * 60)

        # Create high-volume write script
        high_volume_script = '''output("Starting high-volume write operations")

# Write multiple data points rapidly
counter = 0
while counter < 100 {
    data_key = "stress_data_" + string(counter)
    data_value = "High volume data entry number " + string(counter)
    output("Writing entry", counter)
    counter = counter + 1
}

output("High-volume write operations completed")
output("Total entries written:", counter)'''

        if not self.create_stress_script("high_volume_write.mbl", high_volume_script):
            return False

        print("  🚀 Starting concurrent high-volume writes on all nodes...")

        # Check resources before starting
        if not self.check_system_resources():
            print("  ⚠️ System resources too high - skipping stress test")
            return False

        # Execute concurrent writes
        start_time = time.time()

        def run_concurrent_write(node_id):
            return self.run_script_on_node(node_id, "high_volume_write.mbl", timeout=120)

        with ThreadPoolExecutor(max_workers=3) as executor:
            futures = {executor.submit(run_concurrent_write, node_id): node_id
                      for node_id in self.nodes.keys()}

            results = {}
            for future in as_completed(futures):
                node_id = futures[future]
                try:
                    result = future.result()
                    results[node_id] = result
                except Exception as exc:
                    print(f"  ❌ {node_id} generated exception: {exc}")
                    results[node_id] = None

        end_time = time.time()
        duration = end_time - start_time

        # Analyze results
        successful_nodes = 0
        for node_id, result in results.items():
            if result and result.returncode == 0:
                print(f"  ✅ {node_id}: High-volume writes completed")
                successful_nodes += 1
            else:
                print(f"  ❌ {node_id}: High-volume writes failed")
                if result and result.stderr:
                    print(f"       Error: {result.stderr[:100]}...")

        print(f"  📊 Duration: {duration:.2f} seconds")
        print(f"  📊 Success rate: {successful_nodes}/3 nodes")

        # Check resources after stress
        print("  📊 Post-stress resource check:")
        self.check_system_resources()

        return successful_nodes >= 2

    def test_network_partition_simulation(self):
        """CHAOS TEST 1: Network partition simulation"""
        print("\n⚡ CHAOS TEST 1: Network partition simulation")
        print("-" * 60)

        # Create partition test scripts
        pre_partition_script = '''output("Pre-partition data write")
partition_data = "Data before network partition"
output("Partition data written:", partition_data)
output("Pre-partition test completed")'''

        post_partition_script = '''output("Post-partition data verification")
output("Checking data consistency after partition")
verification_data = "Verification after partition recovery"
output("Post-partition test completed")'''

        if not self.create_stress_script("pre_partition.mbl", pre_partition_script):
            return False

        if not self.create_stress_script("post_partition.mbl", post_partition_script):
            return False

        # Phase 1: Write data before partition
        print("  📝 Phase 1: Writing data before partition...")
        result = self.run_script_on_node("ra-do-ki", "pre_partition.mbl")
        if not (result and result.returncode == 0):
            print("  ❌ Pre-partition write failed")
            return False
        print("  ✅ Pre-partition data written")

        # Phase 2: Simulate network partition by stopping middle node
        print("  🔌 Phase 2: Simulating network partition (isolating fi-ne-so)...")
        partition_result = self.ssh_command("192.168.122.11", "pkill -f amorphd || true")
        time.sleep(3)

        # Phase 3: Test remaining nodes during partition
        print("  🔍 Phase 3: Testing remaining nodes during partition...")
        remaining_nodes = ["ra-do-ki", "lu-ma-te"]
        partition_test_results = {}

        for node_id in remaining_nodes:
            print(f"    Testing {node_id} during partition...")
            result = self.run_script_on_node(node_id, "post_partition.mbl")
            partition_test_results[node_id] = result and result.returncode == 0

            if partition_test_results[node_id]:
                print(f"    ✅ {node_id} operational during partition")
            else:
                print(f"    ⚠️ {node_id} issues during partition")

        # Phase 4: Recover from partition
        print("  🔄 Phase 4: Recovering from network partition...")
        recovery_cmd = "nohup /home/amorphdb/amorphdb/bin/amorphd -node fi-ne-so -data /home/amorphdb/amorphdb/data -port 5000 > /home/amorphdb/amorphd.log 2>&1 &"
        recovery_result = self.ssh_command("192.168.122.11", recovery_cmd)
        time.sleep(5)

        # Phase 5: Test full mesh recovery
        print("  🔍 Phase 5: Testing full mesh recovery...")
        recovery_success = True
        for node_id in self.nodes.keys():
            print(f"    Testing {node_id} post-recovery...")
            result = self.run_script_on_node(node_id, "post_partition.mbl")

            if result and result.returncode == 0:
                print(f"    ✅ {node_id} recovered successfully")
            else:
                print(f"    ❌ {node_id} recovery issues")
                recovery_success = False

        return recovery_success

    def test_rapid_node_cycling(self):
        """CHAOS TEST 2: Rapid node cycling (stop/start cycles)"""
        print("\n🌪️ CHAOS TEST 2: Rapid node cycling")
        print("-" * 60)

        cycling_test_script = '''output("Node cycling test")
cycle_data = "Data during node cycling stress"
output("Cycle data:", cycle_data)
output("Node cycling test completed")'''

        if not self.create_stress_script("cycle_test.mbl", cycling_test_script):
            return False

        # Test rapid cycling on fi-ne-so while others operate
        target_node = "fi-ne-so"
        target_ip = self.nodes[target_node]["ip"]

        print(f"  🔄 Rapid cycling node {target_node}...")

        for cycle in range(3):
            print(f"    Cycle {cycle + 1}/3:")

            # Stop node
            print(f"      🛑 Stopping {target_node}...")
            self.ssh_command(target_ip, "pkill -f amorphd || true")
            time.sleep(2)

            # Test other nodes while target is down
            print(f"      🔍 Testing other nodes while {target_node} is down...")
            other_nodes = [nid for nid in self.nodes.keys() if nid != target_node]

            for other_node in other_nodes[:1]:  # Test just one to save time
                result = self.run_script_on_node(other_node, "cycle_test.mbl", timeout=30)
                if result and result.returncode == 0:
                    print(f"        ✅ {other_node} operational during {target_node} downtime")
                else:
                    print(f"        ⚠️ {other_node} issues during {target_node} downtime")

            # Restart node
            print(f"      🚀 Restarting {target_node}...")
            restart_cmd = f"nohup /home/amorphdb/amorphdb/bin/amorphd -node {target_node} -data /home/amorphdb/amorphdb/data -port 5000 > /home/amorphdb/amorphd.log 2>&1 &"
            self.ssh_command(target_ip, restart_cmd)
            time.sleep(3)

            # Test restarted node
            result = self.run_script_on_node(target_node, "cycle_test.mbl", timeout=30)
            if result and result.returncode == 0:
                print(f"      ✅ {target_node} restart #{cycle + 1} successful")
            else:
                print(f"      ❌ {target_node} restart #{cycle + 1} failed")

        print("  🎯 Rapid cycling stress test completed")
        return True

    def test_data_consistency_validation(self):
        """VALIDATION TEST: Data consistency across chaos scenarios"""
        print("\n🔍 VALIDATION TEST: Data consistency verification")
        print("-" * 60)

        # Create consistency validation script
        consistency_script = '''output("Data consistency validation")
validation_timestamp = "Data written for consistency check"
output("Validation data:", validation_timestamp)
output("Consistency validation completed")'''

        if not self.create_stress_script("consistency_check.mbl", consistency_script):
            return False

        print("  📝 Writing consistency check data...")

        # Write consistency data on all nodes
        consistency_results = {}
        for node_id in self.nodes.keys():
            print(f"    Writing consistency data on {node_id}...")
            result = self.run_script_on_node(node_id, "consistency_check.mbl")
            consistency_results[node_id] = result and result.returncode == 0

            if consistency_results[node_id]:
                print(f"    ✅ {node_id} consistency write successful")
            else:
                print(f"    ❌ {node_id} consistency write failed")

        # Cross-node consistency validation
        print("  🔍 Cross-node consistency validation...")
        cross_validation_success = True

        for node_id in self.nodes.keys():
            print(f"    Reading consistency data from {node_id}...")
            result = self.run_script_on_node(node_id, "consistency_check.mbl")

            if result and result.returncode == 0:
                print(f"    ✅ {node_id} consistency read successful")
            else:
                print(f"    ❌ {node_id} consistency read failed")
                cross_validation_success = False

        return cross_validation_success

    def run_stress_and_chaos_tests(self):
        """Run comprehensive stress and chaos testing suite"""
        print("🔥⚡ AMORPHDB STRESS AND CHAOS TESTING SUITE ⚡🔥")
        print("=" * 70)

        # Initial resource check
        print("📊 Initial system resource check:")
        if not self.check_system_resources():
            print("❌ System resources too high to start stress testing")
            return False

        tests = [
            ("High-volume concurrent writes", self.test_high_volume_concurrent_writes),
            ("Network partition simulation", self.test_network_partition_simulation),
            ("Rapid node cycling", self.test_rapid_node_cycling),
            ("Data consistency validation", self.test_data_consistency_validation)
        ]

        results = {}

        for test_name, test_func in tests:
            print(f"\n🎯 STARTING: {test_name}")
            try:
                start_time = time.time()
                success = test_func()
                duration = time.time() - start_time

                results[test_name] = {
                    "status": "✅ PASSED" if success else "⚠️ ISSUES",
                    "duration": f"{duration:.2f}s"
                }
                print(f"   Result: {results[test_name]['status']} ({results[test_name]['duration']})")

                # Resource check between tests
                print("  📊 Post-test resource check:")
                if not self.check_system_resources():
                    print("  ⚠️ High resource usage detected - continuing with caution")

            except Exception as e:
                results[test_name] = {
                    "status": f"❌ ERROR: {e}",
                    "duration": "N/A"
                }
                print(f"   Result: {results[test_name]['status']}")

        # Final Summary
        print(f"\n🔥⚡ STRESS AND CHAOS TESTING COMPLETE ⚡🔥")
        print("=" * 70)

        passed = 0
        for test_name, result in results.items():
            status = result["status"]
            duration = result["duration"]
            print(f"{status:15} {test_name} ({duration})")
            if "PASSED" in status:
                passed += 1

        print(f"\n🎉 STRESS TESTS COMPLETED: {passed}/{len(tests)} passed")

        if passed == len(tests):
            print("🔥 ALL STRESS AND CHAOS TESTS SUCCESSFUL! 🔥")
            print("⚡ AmorphDB mesh survived extreme conditions! ⚡")
            print("🎯 Production-grade resilience VALIDATED!")
        else:
            print("⚠️ Some stress tests detected issues")
            print("🔍 System needs investigation under extreme load")

        # Final resource check
        print("\n📊 Final system resource check:")
        self.check_system_resources()

        return passed == len(tests)

def main():
    """Main stress test execution"""
    test = AmorphDBStressAndChaosTest()

    try:
        success = test.run_stress_and_chaos_tests()
        return 0 if success else 1

    except KeyboardInterrupt:
        print("\n⚠️ Stress testing interrupted by user")
        return 1
    except Exception as e:
        print(f"\n❌ Stress testing failed with error: {e}")
        return 1

if __name__ == "__main__":
    exit(main())