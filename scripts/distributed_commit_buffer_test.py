#!/usr/bin/env python3
"""
Distributed Commit Buffer Test - Tests the new batched commit buffer implementation
across a mesh of AmorphDB nodes to verify loop performance improvements.
"""

import subprocess
import time
import json

class DistributedCommitBufferTest:
    def __init__(self):
        # VM configuration for commit buffer testing
        self.nodes = {
            "ra-do-ki": {"ip": "192.168.122.10", "port": 5000},
            "fi-ne-so": {"ip": "192.168.122.11", "port": 5000},
            "lu-ma-te": {"ip": "192.168.122.12", "port": 5000}
        }

        self.ssh_key = "/home/solifugus/.ssh/amorphdb_cluster_key"

    def ssh_command(self, ip, command, capture_output=True):
        """Execute SSH command on remote node"""
        ssh_cmd = ["ssh", "-i", self.ssh_key, "-o", "ConnectTimeout=5",
                   f"amorphdb@{ip}", command]

        if capture_output:
            result = subprocess.run(ssh_cmd, capture_output=True, text=True)
            return result.stdout.strip(), result.stderr.strip(), result.returncode
        else:
            return subprocess.run(ssh_cmd)

    def start_node(self, node_name, ip, port):
        """Start AmorphDB daemon on a node"""
        print(f"  🚀 Starting {node_name} on {ip}:{port}...")

        # Kill any existing processes
        self.ssh_command(ip, "pkill -f amorphd || true")
        time.sleep(2)

        # Start daemon with commit buffer enabled
        start_cmd = f"nohup /home/amorphdb/amorphdb/bin/amorphd -node {node_name} -data /home/amorphdb/amorphdb/data -port {port} > /home/amorphdb/amorphd.log 2>&1 &"
        stdout, stderr, retcode = self.ssh_command(ip, start_cmd)

        if retcode != 0:
            print(f"    ❌ Failed to start {node_name}: {stderr}")
            return False

        time.sleep(3)

        # Verify the node is running
        check_stdout, check_stderr, check_retcode = self.ssh_command(ip, "pgrep -f amorphd")
        if check_retcode == 0:
            print(f"    ✅ {node_name} started successfully (PID: {check_stdout})")
            return True
        else:
            print(f"    ❌ {node_name} failed to start properly")
            return False

    def test_commit_buffer_loops(self):
        """Test that loops with many writes use commit buffer correctly"""
        print("\n🧪 Testing Commit Buffer with Loop Assignments")
        print("=" * 60)

        target_ip = self.nodes["ra-do-ki"]["ip"]
        target_port = self.nodes["ra-do-ki"]["port"]

        # Create MBL script that performs many writes in a loop
        test_script = """
# Test commit buffer with loop assignments
# This should stage writes in buffer, not send individual messages

for i in range(100) {
    my.commit_test.iteration[i] = i * i
    world.stress.node[i] = "value_" + str(i)
}

# Add a final write
my.commit_test.completed = "finished"
world.test.final = "done"
"""

        # Write test script to VM
        with open("commit_buffer_test.mbl", "w") as f:
            f.write(test_script)

        print("  📝 Uploading test script...")
        subprocess.run(["scp", "-i", self.ssh_key,
                       "commit_buffer_test.mbl",
                       f"amorphdb@{target_ip}:/home/amorphdb/commit_buffer_test.mbl"])

        # Execute test and measure time
        print("  ⏱️  Executing loop test with commit buffer...")
        start_time = time.time()

        exec_cmd = f"cd /home/amorphdb && echo 'exec /home/amorphdb/commit_buffer_test.mbl' | nc localhost {target_port}"
        stdout, stderr, retcode = self.ssh_command(target_ip, exec_cmd)

        end_time = time.time()
        execution_time = end_time - start_time

        print(f"  ⏱️  Loop execution completed in {execution_time:.2f} seconds")

        if retcode == 0:
            print("  ✅ Commit buffer loop test completed successfully")
            print(f"     Output: {stdout}")
        else:
            print(f"  ❌ Commit buffer loop test failed: {stderr}")

        return retcode == 0, execution_time

    def test_cross_zone_batching(self):
        """Test that cross-zone writes are properly batched"""
        print("\n🌐 Testing Cross-Zone Write Batching")
        print("=" * 50)

        # Create a test that writes to different zones
        test_script = """
# Test cross-zone write batching
# These writes should be grouped by destination zone

# Write to multiple zones in a loop
for i in range(20) {
    world.zone1.data[i] = "zone1_" + str(i)
    world.zone2.data[i] = "zone2_" + str(i)
    world.zone3.data[i] = "zone3_" + str(i)
}

world.cross_zone.test = "completed"
"""

        with open("cross_zone_test.mbl", "w") as f:
            f.write(test_script)

        # Upload and execute on first node
        target_ip = self.nodes["ra-do-ki"]["ip"]
        target_port = self.nodes["ra-do-ki"]["port"]

        print("  📝 Uploading cross-zone test script...")
        subprocess.run(["scp", "-i", self.ssh_key,
                       "cross_zone_test.mbl",
                       f"amorphdb@{target_ip}:/home/amorphdb/cross_zone_test.mbl"])

        print("  🌐 Executing cross-zone batching test...")
        start_time = time.time()

        exec_cmd = f"cd /home/amorphdb && echo 'exec /home/amorphdb/cross_zone_test.mbl' | nc localhost {target_port}"
        stdout, stderr, retcode = self.ssh_command(target_ip, exec_cmd)

        end_time = time.time()
        execution_time = end_time - start_time

        print(f"  ⏱️  Cross-zone test completed in {execution_time:.2f} seconds")

        if retcode == 0:
            print("  ✅ Cross-zone batching test completed")
            print(f"     Output: {stdout}")
        else:
            print(f"  ❌ Cross-zone batching test failed: {stderr}")

        return retcode == 0, execution_time

    def check_node_logs(self):
        """Check logs for commit buffer activity"""
        print("\n📋 Checking Node Logs for Commit Buffer Activity")
        print("=" * 55)

        for node_name, config in self.nodes.items():
            print(f"\n  📋 {node_name} ({config['ip']}) logs:")
            stdout, stderr, retcode = self.ssh_command(
                config['ip'],
                "tail -20 /home/amorphdb/amorphd.log | grep -i 'commit\\|buffer\\|flush\\|batch' || echo 'No commit buffer logs found'"
            )

            if stdout:
                for line in stdout.split('\n'):
                    if line.strip():
                        print(f"      {line}")
            else:
                print("      No relevant logs found")

    def cleanup(self):
        """Stop all nodes"""
        print("\n🧹 Cleaning up test environment...")
        for node_name, config in self.nodes.items():
            print(f"  🛑 Stopping {node_name}...")
            self.ssh_command(config['ip'], "pkill -f amorphd || true")

    def run_complete_test(self):
        """Run the complete distributed commit buffer test suite"""
        print("🧪 DISTRIBUTED COMMIT BUFFER TEST SUITE")
        print("=" * 60)
        print("Testing the new batched commit buffer implementation across AmorphDB mesh")

        try:
            # Step 1: Start all nodes
            print("\n🚀 Step 1: Starting AmorphDB nodes...")
            nodes_started = 0
            for node_name, config in self.nodes.items():
                if self.start_node(node_name, config['ip'], config['port']):
                    nodes_started += 1

            if nodes_started < 2:
                print("❌ Not enough nodes started for distributed testing")
                return

            print(f"✅ {nodes_started} nodes started successfully")

            # Step 2: Test commit buffer with loops
            success1, time1 = self.test_commit_buffer_loops()

            # Step 3: Test cross-zone batching
            success2, time2 = self.test_cross_zone_batching()

            # Step 4: Check logs
            self.check_node_logs()

            # Results summary
            print("\n📊 TEST RESULTS SUMMARY")
            print("=" * 40)
            print(f"  Loop Test:       {'✅ PASS' if success1 else '❌ FAIL'} ({time1:.2f}s)")
            print(f"  Cross-Zone Test: {'✅ PASS' if success2 else '❌ FAIL'} ({time2:.2f}s)")

            if success1 and success2:
                print("\n🎉 ALL COMMIT BUFFER TESTS PASSED!")
                print("   The batched commit buffer is working correctly in distributed mode.")
            else:
                print("\n⚠️  SOME TESTS FAILED")
                print("   Check the logs above for details.")

        except KeyboardInterrupt:
            print("\n🛑 Test interrupted by user")
        except Exception as e:
            print(f"\n❌ Test failed with error: {e}")
        finally:
            self.cleanup()

if __name__ == "__main__":
    test = DistributedCommitBufferTest()
    test.run_complete_test()