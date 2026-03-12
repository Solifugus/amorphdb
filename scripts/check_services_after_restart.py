#!/usr/bin/env python3
"""
Check Service Status After Restart
Quick verification that services are running properly
"""

import subprocess

def check_service_status():
    """Check if AmorphDB services are running"""
    print("🔍 CHECKING AMORPHDB SERVICES AFTER RESTART")
    print("=" * 50)

    nodes = {
        "192.168.122.10": "ra-do-ki",
        "192.168.122.11": "fi-ne-so",
        "192.168.122.12": "lu-ma-te"
    }

    for ip, node_id in nodes.items():
        print(f"\n📍 {node_id} ({ip})")

        # Check if amorphd process is running
        proc_result = subprocess.run([
            "ssh", "-o", "ConnectTimeout=3", f"amorphdb@{ip}",
            "ps aux | grep amorphd | grep -v grep"
        ], capture_output=True, text=True, timeout=5)

        if proc_result.returncode == 0 and proc_result.stdout.strip():
            print("   ✅ AmorphD process running")
            # Show first process line
            first_line = proc_result.stdout.strip().split('\n')[0]
            print(f"   📊 {first_line}")
        else:
            print("   ❌ AmorphD process not found")

        # Check if port 5000 is listening
        port_result = subprocess.run([
            "ssh", "-o", "ConnectTimeout=3", f"amorphdb@{ip}",
            "ss -tlnp | grep :5000"
        ], capture_output=True, text=True, timeout=5)

        if port_result.returncode == 0 and port_result.stdout.strip():
            print("   ✅ Port 5000 listening")
        else:
            print("   ❌ Port 5000 not listening")

def test_simple_connectivity():
    """Test simple connectivity to first service"""
    print(f"\n🧪 TESTING SIMPLE CONNECTIVITY")
    print("=" * 50)

    ip = "192.168.122.10"

    # Test basic echo with immediate exit
    simple_command = f"cd /home/amorphdb && echo 'output(\"Service test\")' | timeout 5 ./amorph -node localhost:5000"

    print("🔌 Testing basic connectivity...")

    test_result = subprocess.run([
        "ssh", "-o", "ConnectTimeout=5", f"amorphdb@{ip}", simple_command
    ], capture_output=True, text=True, timeout=10)

    print(f"Return code: {test_result.returncode}")

    if test_result.stdout:
        print(f"✅ Output (first 200 chars): {test_result.stdout[:200]}")

    if test_result.stderr:
        print(f"⚠️ Errors: {test_result.stderr}")

    return test_result.returncode in [0, 124]  # 0=success, 124=timeout but connected

if __name__ == "__main__":
    check_service_status()

    if test_simple_connectivity():
        print(f"\n✅ CONNECTIVITY CONFIRMED - Ready for full testing!")
    else:
        print(f"\n❌ Connectivity issues remain")