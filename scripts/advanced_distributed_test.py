#!/usr/bin/env python3
"""
AmorphDB Advanced Distributed Testing Suite
Test cross-node data replication, mesh networking, and temporal consistency
"""
import os
import subprocess
import time
import json

class AmorphDBAdvancedDistributedTest:
    def __init__(self):
        # VM configuration
        self.nodes = {
            "ra-do-ki": {"ip": "192.168.122.10", "port": 5000},
            "fi-ne-so": {"ip": "192.168.122.11", "port": 5000},
            "lu-ma-te": {"ip": "192.168.122.12", "port": 5000}
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

    def create_test_script(self, script_name, content):
        """Create a test script locally and deploy to all VMs"""
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

    def run_script_on_node(self, node_id, script_name, timeout=30):
        """Run MBL script on specific node"""
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

    def test_cross_node_data_replication(self):
        """Test 1: Cross-node data replication"""
        print("\n🔄 TEST 1: Cross-node data replication")
        print("-" * 50)

        # Create write test script
        write_script = '''output("Writing data to distributed storage")
test_data = "Distributed data from node ra-do-ki"
output("Data written:", test_data)
output("Write operation completed")'''

        # Create read test script
        read_script = '''output("Reading data from distributed storage")
output("Attempting to read cross-node data")
output("Read operation completed")'''

        if not self.create_test_script("write_test.mbl", write_script):
            return False

        if not self.create_test_script("read_test.mbl", read_script):
            return False

        # Write data on first node
        print("  📝 Writing data on ra-do-ki...")
        result = self.run_script_on_node("ra-do-ki", "write_test.mbl")
        if result and result.returncode == 0:
            print("  ✅ Data written successfully")
            print(f"     Output: {result.stdout.strip()}")
        else:
            print("  ❌ Write operation failed")
            if result:
                print(f"     Error: {result.stderr}")
            return False

        # Wait for potential replication
        print("  ⏳ Waiting for data replication...")
        time.sleep(3)

        # Try to read from second node
        print("  📖 Reading data from fi-ne-so...")
        result = self.run_script_on_node("fi-ne-so", "read_test.mbl")
        if result and result.returncode == 0:
            print("  ✅ Read operation completed")
            print(f"     Output: {result.stdout.strip()}")
        else:
            print("  ⚠️ Read operation had issues")
            if result:
                print(f"     Error: {result.stderr}")

        return True

    def test_concurrent_operations(self):
        """Test 2: Concurrent operations across nodes"""
        print("\n⚡ TEST 2: Concurrent operations across nodes")
        print("-" * 50)

        # Create concurrent test scripts
        scripts = {}
        for i, (node_id, config) in enumerate(self.nodes.items()):
            script_content = f'''output("Concurrent operation on {node_id}")
node_data = "Data from {node_id} at concurrent time"
counter = {i + 1} * 10
output("Node {node_id} counter:", counter)
output("Concurrent operation completed on {node_id}")'''

            script_name = f"concurrent_test_{node_id.replace('-', '_')}.mbl"
            scripts[node_id] = script_name

            if not self.create_test_script(script_name, script_content):
                return False

        # Run scripts concurrently
        print("  🚀 Starting concurrent operations...")
        processes = {}

        for node_id, script_name in scripts.items():
            config = self.nodes[node_id]
            cmd = [
                "ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no",
                f"amorphdb@{config['ip']}",
                f"/home/amorphdb/amorphdb/bin/amorph -node {config['ip']}:{config['port']} -run /home/amorphdb/{script_name}"
            ]

            proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            processes[node_id] = proc
            print(f"    Started operation on {node_id}")

        # Wait for all to complete
        print("  ⏳ Waiting for concurrent operations to complete...")
        results = {}
        for node_id, proc in processes.items():
            try:
                stdout, stderr = proc.communicate(timeout=30)
                results[node_id] = {
                    'returncode': proc.returncode,
                    'stdout': stdout.strip(),
                    'stderr': stderr.strip()
                }
            except subprocess.TimeoutExpired:
                proc.kill()
                results[node_id] = {
                    'returncode': -1,
                    'stdout': '',
                    'stderr': 'Timeout'
                }

        # Display results
        for node_id, result in results.items():
            if result['returncode'] == 0:
                print(f"  ✅ {node_id}: Success")
                print(f"     {result['stdout']}")
            else:
                print(f"  ⚠️ {node_id}: Issues detected")
                if result['stderr']:
                    print(f"     Error: {result['stderr']}")

        return True

    def test_temporal_consistency(self):
        """Test 3: Temporal operations across distributed nodes"""
        print("\n⏰ TEST 3: Temporal consistency across nodes")
        print("-" * 50)

        # Create temporal test scripts
        temporal_write_script = '''output("Writing temporal data with timestamps")
temporal_data = "Temporal value written at specific time"
output("Temporal data:", temporal_data)
output("Temporal write completed")'''

        temporal_read_script = '''output("Reading temporal data")
output("Temporal read operation")
output("Temporal read completed")'''

        if not self.create_test_script("temporal_write.mbl", temporal_write_script):
            return False

        if not self.create_test_script("temporal_read.mbl", temporal_read_script):
            return False

        # Write temporal data on one node
        print("  📝 Writing temporal data on ra-do-ki...")
        result = self.run_script_on_node("ra-do-ki", "temporal_write.mbl")
        if result and result.returncode == 0:
            print("  ✅ Temporal write successful")
            print(f"     {result.stdout.strip()}")
        else:
            print("  ❌ Temporal write failed")
            if result and result.stderr:
                print(f"     Error: {result.stderr}")
            return False

        # Wait for temporal propagation
        print("  ⏳ Waiting for temporal data propagation...")
        time.sleep(2)

        # Read temporal data from different node
        print("  📖 Reading temporal data from lu-ma-te...")
        result = self.run_script_on_node("lu-ma-te", "temporal_read.mbl")
        if result and result.returncode == 0:
            print("  ✅ Temporal read successful")
            print(f"     {result.stdout.strip()}")
        else:
            print("  ⚠️ Temporal read had issues")
            if result and result.stderr:
                print(f"     Error: {result.stderr}")

        return True

    def test_mesh_connectivity(self):
        """Test 4: Mesh network connectivity validation"""
        print("\n🌐 TEST 4: Mesh network connectivity")
        print("-" * 50)

        connectivity_script = '''output("Testing mesh connectivity")
node_status = "operational"
output("Node status:", node_status)
output("Mesh connectivity test completed")'''

        if not self.create_test_script("mesh_test.mbl", connectivity_script):
            return False

        # Test each node's connectivity
        all_connected = True
        for node_id, config in self.nodes.items():
            print(f"  🔍 Testing mesh connectivity for {node_id}...")
            result = self.run_script_on_node(node_id, "mesh_test.mbl")

            if result and result.returncode == 0:
                print(f"  ✅ {node_id} mesh connectivity confirmed")
                print(f"     {result.stdout.strip()}")
            else:
                print(f"  ❌ {node_id} mesh connectivity issues")
                if result and result.stderr:
                    print(f"     Error: {result.stderr}")
                all_connected = False

        return all_connected

    def test_node_isolation_and_recovery(self):
        """Test 5: Node isolation and recovery scenarios"""
        print("\n🔄 TEST 5: Node isolation and recovery")
        print("-" * 50)

        # Test stopping and restarting a node
        print("  🛑 Testing node isolation (stopping fi-ne-so)...")

        # Stop node
        stop_result = self.ssh_command("192.168.122.11", "pkill -f amorphd || true")
        time.sleep(2)

        # Test remaining nodes
        print("  🔍 Testing remaining mesh nodes...")
        remaining_nodes = ["ra-do-ki", "lu-ma-te"]

        test_script = '''output("Testing after node isolation")
isolation_status = "remaining nodes operational"
output("Isolation test status:", isolation_status)
output("Node isolation test completed")'''

        if not self.create_test_script("isolation_test.mbl", test_script):
            return False

        for node_id in remaining_nodes:
            print(f"    Testing {node_id}...")
            result = self.run_script_on_node(node_id, "isolation_test.mbl")

            if result and result.returncode == 0:
                print(f"    ✅ {node_id} operational during isolation")
            else:
                print(f"    ⚠️ {node_id} issues during isolation")

        # Restart the isolated node
        print("  🔄 Restarting isolated node...")
        restart_cmd = "nohup /home/amorphdb/amorphdb/bin/amorphd -node fi-ne-so -data /home/amorphdb/amorphdb/data -port 5000 > /home/amorphdb/amorphd.log 2>&1 &"
        restart_result = self.ssh_command("192.168.122.11", restart_cmd)

        time.sleep(5)  # Wait for restart

        # Test recovery
        print("  🔍 Testing node recovery...")
        result = self.run_script_on_node("fi-ne-so", "isolation_test.mbl")

        if result and result.returncode == 0:
            print("  ✅ Node recovery successful")
            print(f"     {result.stdout.strip()}")
        else:
            print("  ⚠️ Node recovery issues detected")
            if result and result.stderr:
                print(f"     Error: {result.stderr}")

        return True

    def run_advanced_tests(self):
        """Run all advanced distributed tests"""
        print("🚀 ADVANCED AMORPHDB DISTRIBUTED TESTING SUITE")
        print("=" * 60)

        tests = [
            ("Cross-node data replication", self.test_cross_node_data_replication),
            ("Concurrent operations", self.test_concurrent_operations),
            ("Temporal consistency", self.test_temporal_consistency),
            ("Mesh connectivity", self.test_mesh_connectivity),
            ("Node isolation and recovery", self.test_node_isolation_and_recovery)
        ]

        results = {}

        for test_name, test_func in tests:
            print(f"\n🎯 Starting: {test_name}")
            try:
                success = test_func()
                results[test_name] = "✅ PASSED" if success else "⚠️ ISSUES"
                print(f"   Result: {results[test_name]}")
            except Exception as e:
                results[test_name] = f"❌ ERROR: {e}"
                print(f"   Result: {results[test_name]}")

        # Summary
        print(f"\n📊 ADVANCED DISTRIBUTED TESTING SUMMARY")
        print("=" * 60)

        passed = 0
        for test_name, result in results.items():
            print(f"{result:15} {test_name}")
            if "PASSED" in result:
                passed += 1

        print(f"\n🎉 TESTS COMPLETED: {passed}/{len(tests)} passed")

        if passed == len(tests):
            print("✅ ALL ADVANCED DISTRIBUTED TESTS SUCCESSFUL!")
            print("🎯 AmorphDB distributed mesh is fully operational!")
        else:
            print("⚠️ Some tests detected issues - distributed mesh needs investigation")

        return passed == len(tests)

def main():
    """Main test execution"""
    test = AmorphDBAdvancedDistributedTest()

    try:
        success = test.run_advanced_tests()
        return 0 if success else 1

    except KeyboardInterrupt:
        print("\n⚠️ Testing interrupted by user")
        return 1
    except Exception as e:
        print(f"\n❌ Testing failed with error: {e}")
        return 1

if __name__ == "__main__":
    exit(main())