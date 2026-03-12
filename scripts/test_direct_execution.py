#!/usr/bin/env python3
"""
Test Direct MBL Execution Without Daemon
Try running tests directly with amorph binary
"""

import subprocess
import time

def test_direct_execution():
    """Test direct execution of MBL script"""
    print("🧪 TESTING DIRECT MBL EXECUTION")
    print("=" * 50)

    # Test on first VM
    ip = "192.168.122.10"
    node = "ra-do-ki"

    print(f"📍 Testing on {node} ({ip})")

    # Create simple test script
    simple_test = '''output("=== Direct Execution Test ===")
test.direct.start_time = "@2026-03-10 22:35:00"
test.direct.execution_mode = "standalone"
test.direct.node_id = "ra-do-ki"
output("Direct execution working!")
'''

    # Write to local temp file
    with open("/tmp/direct_test.mbl", "w") as f:
        f.write(simple_test)

    print("📤 Deploying test script...")

    # Copy to VM
    scp_result = subprocess.run([
        "scp", "-o", "StrictHostKeyChecking=no",
        "/tmp/direct_test.mbl", f"amorphdb@{ip}:/tmp/"
    ], capture_output=True)

    if scp_result.returncode != 0:
        print("❌ Failed to deploy script")
        return False

    print("✅ Script deployed")

    # Try different execution methods
    methods = [
        ("Pipe input", f"cd /home/amorphdb && cat /tmp/direct_test.mbl | ./amorph"),
        ("File redirect", f"cd /home/amorphdb && ./amorph < /tmp/direct_test.mbl"),
        ("Interactive mode", f"cd /home/amorphdb && echo 'output(\"Hello from amorph\")' | ./amorph")
    ]

    for method_name, command in methods:
        print(f"\n🎯 Trying {method_name}:")
        print(f"   Command: {command}")

        ssh_result = subprocess.run([
            "ssh", "-o", "ConnectTimeout=10", f"amorphdb@{ip}", command
        ], capture_output=True, text=True, timeout=15)

        print(f"   Return code: {ssh_result.returncode}")

        if ssh_result.stdout.strip():
            print(f"   ✅ Output: {ssh_result.stdout.strip()[:200]}")

        if ssh_result.stderr.strip():
            print(f"   ⚠️ Errors: {ssh_result.stderr.strip()[:200]}")

        if ssh_result.returncode == 0:
            print(f"   🎉 {method_name} SUCCESS!")
            return True

    print("\n❌ All execution methods failed")
    return False

def test_p3_script_directly():
    """Try executing one of the P3 test scripts directly"""
    print(f"\n🚀 TESTING P3.1 SCRIPT DIRECT EXECUTION")
    print("=" * 50)

    ip = "192.168.122.10"

    print("📤 Deploying P3.1 test...")

    # Copy P3.1 test to VM
    scp_result = subprocess.run([
        "scp", "-o", "StrictHostKeyChecking=no",
        "test_p3_1_replication_overhead.mbl", f"amorphdb@{ip}:/tmp/"
    ], capture_output=True)

    if scp_result.returncode != 0:
        print("❌ Failed to deploy P3.1 script")
        return False

    print("✅ P3.1 script deployed")

    # Try executing P3.1 directly
    print("🎯 Executing P3.1 via file redirect...")

    exec_command = "cd /home/amorphdb && ./amorph < /tmp/test_p3_1_replication_overhead.mbl"

    ssh_result = subprocess.run([
        "ssh", "-o", "ConnectTimeout=10", f"amorphdb@{ip}", exec_command
    ], capture_output=True, text=True, timeout=30)

    print(f"Return code: {ssh_result.returncode}")

    if ssh_result.stdout.strip():
        print(f"✅ Output: {ssh_result.stdout.strip()[:300]}...")

    if ssh_result.stderr.strip():
        print(f"⚠️ Errors: {ssh_result.stderr.strip()[:300]}")

    success = ssh_result.returncode == 0
    if success:
        print("🎉 P3.1 EXECUTION SUCCESS!")
    else:
        print("❌ P3.1 execution failed")

    return success

if __name__ == "__main__":
    print("🔬 DIRECT EXECUTION TESTING")
    print("Testing whether MBL scripts can run without daemon")

    if test_direct_execution():
        print("\n✅ Direct execution works! Trying P3.1...")

        if test_p3_script_directly():
            print("\n🏆 SUCCESS! P3 tests can run directly!")
            print("💡 Will proceed with direct execution method")
        else:
            print("\n⚠️ P3.1 failed - may need daemon or different approach")
    else:
        print("\n❌ Direct execution not working")
        print("💡 Need to investigate AmorphDB setup further")