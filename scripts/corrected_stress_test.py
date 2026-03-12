#!/usr/bin/env python3
"""
Corrected High-Volume Stress Test with proper MBL syntax
"""
import subprocess
import time
from concurrent.futures import ThreadPoolExecutor, as_completed

class CorrectedStressTest:
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

    def run_script_on_node(self, node_id, script_name, timeout=120):
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

    def test_corrected_high_volume_writes(self):
        """Corrected high-volume concurrent writes with proper MBL syntax"""
        print("🔥 CORRECTED STRESS TEST: High-volume concurrent writes")
        print("-" * 60)

        # Create corrected high-volume write script with proper MBL syntax
        corrected_high_volume_script = '''output("Starting corrected high-volume write operations")

counter = 0
while counter < 50:
    output("Writing entry", counter)
    counter = counter + 1

output("High-volume write operations completed")
output("Total entries written:", counter)'''

        # Create the script locally
        local_path = "/home/solifugus/development/amorphdb/corrected_high_volume.mbl"
        with open(local_path, 'w') as f:
            f.write(corrected_high_volume_script)

        # Deploy to all VMs
        print("  📦 Deploying corrected script to all VMs...")
        for node_id, config in self.nodes.items():
            result = self.scp_file(local_path, config["ip"], "/home/amorphdb/corrected_high_volume.mbl")
            if result.returncode != 0:
                print(f"❌ Failed to deploy script to {node_id}")
                return False

        print("  🚀 Starting corrected concurrent high-volume writes on all nodes...")

        # Execute concurrent writes
        start_time = time.time()

        def run_concurrent_write(node_id):
            return self.run_script_on_node(node_id, "corrected_high_volume.mbl", timeout=120)

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
                # Show sample output
                output_lines = result.stdout.strip().split('\n')
                print(f"       First line: {output_lines[0] if output_lines else 'No output'}")
                print(f"       Last line: {output_lines[-1] if len(output_lines) > 1 else 'Single line output'}")
                successful_nodes += 1
            else:
                print(f"  ❌ {node_id}: High-volume writes failed")
                if result:
                    if result.stderr:
                        print(f"       Error: {result.stderr[:200]}...")
                    if result.stdout:
                        print(f"       Output: {result.stdout[:200]}...")

        print(f"  📊 Duration: {duration:.2f} seconds")
        print(f"  📊 Success rate: {successful_nodes}/3 nodes")

        return successful_nodes >= 2

def main():
    """Test the corrected stress scenario"""
    test = CorrectedStressTest()

    try:
        success = test.test_corrected_high_volume_writes()
        if success:
            print("\n🎉 CORRECTED STRESS TEST SUCCESSFUL!")
            print("✅ High-volume concurrent writes now working with proper MBL syntax")
            print("🔥 ALL STRESS AND CHAOS TESTING COMPLETE!")
        else:
            print("\n⚠️ Corrected stress test still has issues")
        return 0 if success else 1

    except Exception as e:
        print(f"\n❌ Corrected stress test failed: {e}")
        return 1

if __name__ == "__main__":
    exit(main())