#!/usr/bin/env python3
"""
Check AmorphDB Daemon Logs and Status
Understand why daemon startup is failing
"""

import subprocess

def ssh_command(ip, command):
    """Execute SSH command with timeout"""
    ssh_cmd = ["ssh", "-o", "ConnectTimeout=5", f"amorphdb@{ip}", command]
    try:
        result = subprocess.run(ssh_cmd, capture_output=True, text=True, timeout=10)
        return result
    except subprocess.TimeoutExpired:
        return None

def check_node_status(node_id, ip):
    """Check comprehensive status of a node"""
    print(f"\n🔍 CHECKING {node_id} ({ip})")
    print("=" * 40)

    # Check if any amorphd processes running
    print("1. Process check:")
    result = ssh_command(ip, "ps aux | grep amorph")
    if result:
        processes = [line for line in result.stdout.split('\n') if 'amorph' in line and 'grep' not in line]
        if processes:
            print("   ✅ AmorphDB processes found:")
            for proc in processes[:3]:  # First 3 processes
                print(f"      {proc.strip()}")
        else:
            print("   ❌ No AmorphDB processes running")
    else:
        print("   ⏰ Process check timed out")

    # Check recent daemon logs
    print("\n2. Recent daemon logs:")
    for log_file in ["amorphd.log", "amorphd_new.log"]:
        print(f"   📄 {log_file}:")
        result = ssh_command(ip, f"tail -10 /home/amorphdb/{log_file} 2>/dev/null || echo 'No {log_file}'")
        if result and result.stdout.strip():
            for line in result.stdout.strip().split('\n')[:5]:  # First 5 lines
                print(f"      {line}")

    # Check system logs for amorphd
    print("\n3. System logs (last 5 lines):")
    result = ssh_command(ip, "journalctl -u amorphd --no-pager -n 5 2>/dev/null || echo 'No systemd logs'")
    if result and result.stdout.strip():
        for line in result.stdout.strip().split('\n'):
            print(f"      {line}")

    # Check listening ports
    print("\n4. Listening ports:")
    result = ssh_command(ip, "ss -tlnp | grep 5000 || echo 'Port 5000 not listening'")
    if result:
        print(f"   {result.stdout.strip()}")

    # Try manual daemon start
    print("\n5. Manual daemon test:")
    result = ssh_command(ip, f"cd /home/amorphdb && ./amorphd --help 2>&1 | head -3")
    if result:
        if result.returncode == 0:
            print("   ✅ amorphd binary responds to --help")
            print(f"   Output: {result.stdout.strip()}")
        else:
            print("   ❌ amorphd --help failed")
            print(f"   Error: {result.stderr.strip()}")

    # Check configuration files
    print("\n6. Configuration check:")
    result = ssh_command(ip, "ls -la /home/amorphdb/amorphdb/config/")
    if result and result.returncode == 0:
        print("   📁 Config directory contents:")
        for line in result.stdout.strip().split('\n')[1:4]:  # Skip total, show first 3
            print(f"      {line}")
    else:
        print("   ❌ No config directory or access failed")

def main():
    """Check all nodes"""
    print("🔧 AMORPHDB DAEMON STATUS CHECK")
    print("=" * 50)

    nodes = {
        "ra-do-ki": "192.168.122.10",
        "fi-ne-so": "192.168.122.11",
        "lu-ma-te": "192.168.122.12"
    }

    for node_id, ip in nodes.items():
        check_node_status(node_id, ip)

    print("\n" + "=" * 50)
    print("💡 ANALYSIS:")
    print("- If no processes running: Daemon startup failing")
    print("- If port not listening: Service not binding to port")
    print("- Check logs for specific error messages")
    print("- May need specific configuration or startup parameters")

if __name__ == "__main__":
    main()