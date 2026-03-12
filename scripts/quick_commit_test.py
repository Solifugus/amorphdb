#!/usr/bin/env python3
"""
Quick Commit Buffer Test - Simplified test using proper SSH config
"""

import subprocess
import time

def ssh_cmd(node, command):
    """Execute SSH command using the proper config"""
    cmd = ["ssh", "-F", "/home/solifugus/.ssh/amorphdb_config", node, command]
    result = subprocess.run(cmd, capture_output=True, text=True)
    return result.stdout.strip(), result.stderr.strip(), result.returncode

def main():
    print("🧪 Quick Commit Buffer Test")
    print("=" * 40)

    # Test nodes (using proper SSH config names)
    nodes = ["amorphdb0", "amorphdb1", "amorphdb2"]

    # Step 1: Check if AmorphDB binaries are present
    print("\n📋 Checking AmorphDB installation...")
    for node in nodes:
        stdout, stderr, retcode = ssh_cmd(node, "ls -la /home/amorphdb/amorphdb/bin/amorphd")
        if retcode == 0:
            print(f"  ✅ {node}: AmorphDB binary present")
        else:
            print(f"  ❌ {node}: Binary missing - {stderr}")

    # Step 2: Start one node for testing
    test_node = "amorphdb0"
    print(f"\n🚀 Starting {test_node}...")

    # Kill any existing process
    ssh_cmd(test_node, "pkill -f amorphd || true")
    time.sleep(2)

    # Start AmorphDB daemon
    start_cmd = "nohup /home/amorphdb/amorphdb/bin/amorphd -node ra-do-ki -data /home/amorphdb/amorphdb/data -port 5000 > /home/amorphdb/amorphd.log 2>&1 &"
    stdout, stderr, retcode = ssh_cmd(test_node, start_cmd)

    time.sleep(3)

    # Check if it's running
    stdout, stderr, retcode = ssh_cmd(test_node, "pgrep -f amorphd")
    if retcode == 0:
        print(f"  ✅ {test_node} started (PID: {stdout})")
    else:
        print(f"  ❌ {test_node} failed to start")
        return

    # Step 3: Test simple MBL execution with commit buffer
    print("\n📝 Testing MBL execution...")

    # Create simple test script
    test_script = 'my.commit_test.value = "hello_commit_buffer"'

    # Upload script to VM
    with open("/tmp/simple_test.mbl", "w") as f:
        f.write(test_script)

    subprocess.run(["scp", "-F", "/home/solifugus/.ssh/amorphdb_config",
                   "/tmp/simple_test.mbl", f"{test_node}:/home/amorphdb/simple_test.mbl"])

    # Execute via netcat
    exec_cmd = "cd /home/amorphdb && echo 'exec /home/amorphdb/simple_test.mbl' | nc localhost 5000"
    stdout, stderr, retcode = ssh_cmd(test_node, exec_cmd)

    if retcode == 0:
        print(f"  ✅ MBL execution successful: {stdout}")
    else:
        print(f"  ❌ MBL execution failed: {stderr}")

    # Step 4: Check logs for commit buffer activity
    print("\n📋 Checking logs...")
    stdout, stderr, retcode = ssh_cmd(test_node, "tail -10 /home/amorphdb/amorphd.log")
    if stdout:
        print("  Recent logs:")
        for line in stdout.split('\n')[-5:]:
            if line.strip():
                print(f"    {line}")

    # Cleanup
    print("\n🧹 Cleanup...")
    ssh_cmd(test_node, "pkill -f amorphd || true")

    print("\n✅ Quick test completed!")

if __name__ == "__main__":
    main()