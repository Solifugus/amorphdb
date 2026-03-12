#!/usr/bin/env python3
"""
Test Minimal Connectivity
Start with the simplest possible MBL script to verify basic functionality
"""

import subprocess
import time

def test_minimal_script():
    """Test with minimal MBL script"""
    print("🔬 TESTING MINIMAL MBL CONNECTIVITY")
    print("=" * 50)

    # Create minimal test script
    minimal_test = '''output("Hello from AmorphDB!")
test.minimal = true
output("Minimal test completed")
'''

    with open("/tmp/minimal_test.mbl", "w") as f:
        f.write(minimal_test)

    ip = "192.168.122.10"
    port = 5000

    print(f"📍 Testing on {ip}:{port}")

    # Deploy script
    print("📤 Deploying minimal script...")
    scp_result = subprocess.run([
        "scp", "-o", "StrictHostKeyChecking=no",
        "/tmp/minimal_test.mbl", f"amorphdb@{ip}:/tmp/"
    ], capture_output=True, timeout=10)

    if scp_result.returncode != 0:
        print("❌ Deploy failed")
        return False

    print("✅ Script deployed")

    # Try execution
    print("🚀 Executing minimal script...")

    exec_command = f"cd /home/amorphdb && timeout 15 ./amorph -node localhost:{port} < /tmp/minimal_test.mbl"

    exec_result = subprocess.run([
        "ssh", "-o", "ConnectTimeout=5", f"amorphdb@{ip}", exec_command
    ], capture_output=True, text=True, timeout=20)

    print(f"Return code: {exec_result.returncode}")

    if exec_result.stdout:
        print(f"✅ Output: {exec_result.stdout}")

    if exec_result.stderr:
        print(f"⚠️ Errors: {exec_result.stderr}")

    return exec_result.returncode == 0

def test_even_simpler():
    """Test with even simpler approach"""
    print(f"\n🔬 TESTING EVEN SIMPLER APPROACH")
    print("=" * 50)

    ip = "192.168.122.10"

    # Test direct echo to amorph
    print("📝 Testing echo to amorph...")

    exec_command = f"cd /home/amorphdb && echo 'output(\"Direct echo test\")' | timeout 10 ./amorph -node localhost:5000"

    exec_result = subprocess.run([
        "ssh", "-o", "ConnectTimeout=5", f"amorphdb@{ip}", exec_command
    ], capture_output=True, text=True, timeout=15)

    print(f"Return code: {exec_result.returncode}")

    if exec_result.stdout:
        print(f"✅ Output: {exec_result.stdout}")

    if exec_result.stderr:
        print(f"⚠️ Errors: {exec_result.stderr}")

    return exec_result.returncode == 0

def check_service_status():
    """Check if service is actually responding"""
    print(f"\n🔍 CHECKING SERVICE RESPONSIVENESS")
    print("=" * 50)

    ip = "192.168.122.10"

    # Check if service is responding to connections
    print("🔌 Testing basic connection...")

    test_command = f"telnet localhost 5000 <<< 'quit' || echo 'Connection test complete'"

    exec_result = subprocess.run([
        "ssh", "-o", "ConnectTimeout=5", f"amorphdb@{ip}", test_command
    ], capture_output=True, text=True, timeout=10)

    print(f"Telnet result: {exec_result.returncode}")
    if exec_result.stdout:
        print(f"Output: {exec_result.stdout[:200]}")

    # Check process details
    print("\n🔍 Process details...")
    proc_command = f"ps aux | grep amorphd | grep -v grep"

    proc_result = subprocess.run([
        "ssh", "-o", "ConnectTimeout=5", f"amorphdb@{ip}", proc_command
    ], capture_output=True, text=True, timeout=10)

    if proc_result.stdout:
        print(f"AmorphD process: {proc_result.stdout.strip()}")

def main():
    """Run all tests"""
    print("🧪 MINIMAL CONNECTIVITY TESTING")
    print("Testing basic AmorphDB functionality")

    check_service_status()

    if test_even_simpler():
        print("\n✅ Basic echo test passed!")

        if test_minimal_script():
            print("\n🎉 MINIMAL CONNECTIVITY SUCCESSFUL!")
            print("💡 AmorphDB is working - issue is with complex scripts")
        else:
            print("\n⚠️ Minimal script failed")
    else:
        print("\n❌ Even basic echo failed")
        print("💡 May be service or connection issue")

if __name__ == "__main__":
    main()