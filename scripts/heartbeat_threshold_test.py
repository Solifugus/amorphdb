#!/usr/bin/env python3
"""
AmorphDB Heartbeat Threshold Testing
Systematically test loop sizes to identify heartbeat timeout thresholds
"""
import subprocess
import time

class HeartbeatThresholdTest:
    def __init__(self):
        self.nodes = {
            "ra-do-ki": {"ip": "192.168.122.10", "port": 5000},
            "fi-ne-so": {"ip": "192.168.122.11", "port": 5000},
            "lu-ma-te": {"ip": "192.168.122.12", "port": 5000}
        }

    def scp_file(self, local_path, ip, remote_path):
        """Copy file to remote VM via SCP"""
        scp_cmd = [
            "scp", "-o", "StrictHostKeyChecking=no",
            local_path, f"amorphdb@{ip}:{remote_path}"
        ]
        return subprocess.run(scp_cmd, capture_output=True)

    def run_timed_script(self, node_id, script_name, timeout=60):
        """Run MBL script with precise timing"""
        config = self.nodes[node_id]
        command = f"/home/amorphdb/amorphdb/bin/amorph -node {config['ip']}:{config['port']} -run /home/amorphdb/{script_name}"

        start_time = time.time()

        try:
            result = subprocess.run([
                "ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no",
                f"amorphdb@{config['ip']}", command
            ], capture_output=True, text=True, timeout=timeout)

            end_time = time.time()
            duration = end_time - start_time

            return {
                'success': result.returncode == 0,
                'duration': duration,
                'stdout': result.stdout.strip(),
                'stderr': result.stderr.strip(),
                'returncode': result.returncode
            }

        except subprocess.TimeoutExpired:
            duration = timeout
            return {
                'success': False,
                'duration': duration,
                'stdout': '',
                'stderr': 'Timeout',
                'returncode': -1
            }

    def create_loop_test(self, loop_size, test_name):
        """Create a loop test with specific iteration count"""
        script_content = f'''output("Starting {test_name} with {loop_size} iterations")
start_time = "Test started"
output("Start:", start_time)

counter = 1
while counter <= {loop_size}:
    output("Iteration", counter)
    counter = counter + 1

end_time = "Test completed"
output("End:", end_time)
output("Total iterations completed:", counter - 1)'''

        script_name = f"loop_test_{loop_size}.mbl"
        local_path = f"/home/solifugus/development/amorphdb/{script_name}"

        with open(local_path, 'w') as f:
            f.write(script_content)

        # Deploy to test node
        test_ip = self.nodes["ra-do-ki"]["ip"]
        result = self.scp_file(local_path, test_ip, f"/home/amorphdb/{script_name}")

        return script_name if result.returncode == 0 else None

    def test_single_operations(self):
        """Test single operations vs loops"""
        print("🔍 TESTING: Single operations vs loops")
        print("-" * 50)

        # Test 1: Single output operation
        single_script = '''output("Single operation test")
output("This is a simple single operation")
output("Single operation completed")'''

        script_name = "single_op_test.mbl"
        local_path = f"/home/solifugus/development/amorphdb/{script_name}"

        with open(local_path, 'w') as f:
            f.write(single_script)

        # Deploy and test
        test_ip = self.nodes["ra-do-ki"]["ip"]
        self.scp_file(local_path, test_ip, f"/home/amorphdb/{script_name}")

        result = self.run_timed_script("ra-do-ki", script_name)

        print(f"  Single Operation: {'✅ SUCCESS' if result['success'] else '❌ FAILED'}")
        print(f"    Duration: {result['duration']:.2f}s")
        print(f"    Return code: {result['returncode']}")

        if result['stdout']:
            print(f"    Output: {result['stdout']}")

        return result['success']

    def test_progressive_loop_sizes(self):
        """Test progressively larger loops to find threshold"""
        print("\n⏱️ TESTING: Progressive loop sizes to find heartbeat threshold")
        print("-" * 60)

        # Test various loop sizes
        loop_sizes = [3, 5, 10, 15, 20, 25, 30, 40, 50]
        results = {}

        for loop_size in loop_sizes:
            print(f"  Testing {loop_size} iterations...")

            script_name = self.create_loop_test(loop_size, f"{loop_size}-iteration loop")
            if not script_name:
                print(f"    ❌ Failed to deploy script for {loop_size} iterations")
                continue

            result = self.run_timed_script("ra-do-ki", script_name, timeout=30)
            results[loop_size] = result

            status = "✅ SUCCESS" if result['success'] else "❌ FAILED"
            print(f"    {loop_size:2d} iterations: {status} ({result['duration']:.2f}s)")

            if result['success']:
                # Show first and last output lines
                lines = result['stdout'].split('\n')
                if len(lines) >= 2:
                    print(f"       First: {lines[0]}")
                    print(f"       Last:  {lines[-1]}")
            else:
                print(f"       Error: Return code {result['returncode']}")
                if result['stderr']:
                    print(f"       Stderr: {result['stderr'][:50]}...")

            # Stop testing larger sizes once we hit consistent failures
            if not result['success'] and loop_size >= 20:
                print(f"  ⚠️ Stopping at {loop_size} iterations - consistent failures detected")
                break

        return results

    def test_chunked_operations(self):
        """Test chunked operations with delays between chunks"""
        print("\n🔄 TESTING: Chunked operations with delays")
        print("-" * 50)

        # Create chunked script: 3 chunks of 5 iterations each with delays
        chunked_script = '''output("Starting chunked operations test")

# Chunk 1
output("Starting chunk 1")
counter = 1
while counter <= 5:
    output("Chunk 1, iteration", counter)
    counter = counter + 1
output("Chunk 1 completed")

# Small delay (simulated with simple operations)
delay_counter = 1
while delay_counter <= 2:
    temp = "delay operation"
    delay_counter = delay_counter + 1

# Chunk 2
output("Starting chunk 2")
counter = 1
while counter <= 5:
    output("Chunk 2, iteration", counter)
    counter = counter + 1
output("Chunk 2 completed")

# Another small delay
delay_counter = 1
while delay_counter <= 2:
    temp = "delay operation"
    delay_counter = delay_counter + 1

# Chunk 3
output("Starting chunk 3")
counter = 1
while counter <= 5:
    output("Chunk 3, iteration", counter)
    counter = counter + 1
output("Chunk 3 completed")

output("All chunks completed successfully")'''

        script_name = "chunked_test.mbl"
        local_path = f"/home/solifugus/development/amorphdb/{script_name}"

        with open(local_path, 'w') as f:
            f.write(chunked_script)

        # Deploy and test
        test_ip = self.nodes["ra-do-ki"]["ip"]
        self.scp_file(local_path, test_ip, f"/home/amorphdb/{script_name}")

        result = self.run_timed_script("ra-do-ki", script_name, timeout=45)

        print(f"  Chunked Operations: {'✅ SUCCESS' if result['success'] else '❌ FAILED'}")
        print(f"    Duration: {result['duration']:.2f}s")
        print(f"    Return code: {result['returncode']}")

        if result['success']:
            lines = result['stdout'].split('\n')
            print(f"    First chunk: {lines[0] if lines else 'No output'}")
            print(f"    Last output: {lines[-1] if lines else 'No output'}")

        return result['success']

    def analyze_threshold_results(self, results):
        """Analyze results to identify heartbeat threshold"""
        print("\n📊 HEARTBEAT THRESHOLD ANALYSIS")
        print("=" * 50)

        successful_sizes = [size for size, result in results.items() if result['success']]
        failed_sizes = [size for size, result in results.items() if not result['success']]

        if successful_sizes and failed_sizes:
            max_successful = max(successful_sizes)
            min_failed = min(failed_sizes)

            print(f"✅ Maximum successful loop size: {max_successful} iterations")
            print(f"❌ Minimum failed loop size: {min_failed} iterations")
            print(f"🎯 Heartbeat threshold: Between {max_successful} and {min_failed} iterations")

            # Analyze timing patterns
            successful_times = [results[size]['duration'] for size in successful_sizes]
            failed_times = [results[size]['duration'] for size in failed_sizes if results[size]['duration'] > 0]

            if successful_times:
                avg_success_time = sum(successful_times) / len(successful_times)
                max_success_time = max(successful_times)
                print(f"📈 Successful operation timing:")
                print(f"    Average: {avg_success_time:.2f}s")
                print(f"    Maximum: {max_success_time:.2f}s")

            if failed_times:
                avg_failed_time = sum(failed_times) / len(failed_times)
                print(f"📈 Failed operation timing:")
                print(f"    Average failure time: {avg_failed_time:.2f}s")
                print(f"    🎯 LIKELY HEARTBEAT INTERVAL: ~{avg_failed_time:.0f} seconds")

        elif successful_sizes:
            print(f"✅ All tested sizes succeeded (up to {max(successful_sizes)} iterations)")
            print("🔍 Heartbeat threshold is higher than tested range")

        elif failed_sizes:
            print(f"❌ All tested sizes failed (starting from {min(failed_sizes)} iterations)")
            print("🔍 Heartbeat threshold is lower than tested range")

        else:
            print("⚠️ No test results to analyze")

    def run_heartbeat_threshold_tests(self):
        """Run comprehensive heartbeat threshold testing"""
        print("⏱️🔍 AMORPHDB HEARTBEAT THRESHOLD INVESTIGATION")
        print("=" * 60)

        # Test 1: Baseline single operations
        if not self.test_single_operations():
            print("❌ Basic operations failed - aborting heartbeat testing")
            return False

        # Test 2: Progressive loop sizes
        results = self.test_progressive_loop_sizes()

        # Test 3: Chunked operations
        chunked_success = self.test_chunked_operations()

        # Test 4: Analysis
        self.analyze_threshold_results(results)

        # Summary
        print(f"\n🎯 HEARTBEAT THRESHOLD INVESTIGATION COMPLETE")
        print("=" * 60)

        successful_tests = sum([1 for r in results.values() if r['success']])
        total_tests = len(results)

        print(f"📊 Loop tests: {successful_tests}/{total_tests} succeeded")
        print(f"📊 Chunked operations: {'✅ SUCCESS' if chunked_success else '❌ FAILED'}")

        if any(r['success'] for r in results.values()):
            print("✅ Heartbeat threshold identified!")
            print("🎯 AmorphDB has specific timing limits for sustained operations")
        else:
            print("⚠️ All loop tests failed - may indicate other issues")

        return True

def main():
    """Main test execution"""
    test = HeartbeatThresholdTest()

    try:
        test.run_heartbeat_threshold_tests()
        return 0

    except KeyboardInterrupt:
        print("\n⚠️ Heartbeat testing interrupted by user")
        return 1
    except Exception as e:
        print(f"\n❌ Heartbeat testing failed with error: {e}")
        return 1

if __name__ == "__main__":
    exit(main())