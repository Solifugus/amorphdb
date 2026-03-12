#!/usr/bin/env python3
"""
Diagnose AmorphDB Execution Environment
Check what's different between working and non-working execution
"""

import subprocess
import os
import sys

def run_command(cmd, description):
    """Run a command and return results"""
    print(f"\n=== {description} ===")
    try:
        result = subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=10)
        print(f"Command: {cmd}")
        print(f"Return code: {result.returncode}")
        if result.stdout:
            print(f"STDOUT:\n{result.stdout}")
        if result.stderr:
            print(f"STDERR:\n{result.stderr}")
        return result
    except subprocess.TimeoutExpired:
        print(f"Command timed out: {cmd}")
        return None
    except Exception as e:
        print(f"Error running command: {e}")
        return None

def main():
    print("🔍 DIAGNOSING AMORPHDB EXECUTION ENVIRONMENT")
    print("=" * 60)

    # Check local amorph binary
    run_command("which amorph", "Check amorph in PATH")
    run_command("ls -la ./amorph", "Check local amorph binary")
    run_command("ls -la ./bin/amorph", "Check bin/amorph binary")

    # Check SSH connectivity
    run_command("ssh-keygen -F 192.168.122.10", "Check known hosts for VM IPs")
    run_command("ls -la ~/.ssh/", "Check SSH directory")

    # Check VM connectivity
    for ip in ["192.168.122.10", "192.168.122.11", "192.168.122.12"]:
        run_command(f"ping -c 1 -W 2 {ip}", f"Ping {ip}")
        run_command(f"ssh -o BatchMode=yes -o ConnectTimeout=3 amorphdb@{ip} 'echo connected'", f"SSH test to {ip}")

    # Check if VMs are running
    run_command("virsh list", "Check running VMs")
    run_command("ps aux | grep qemu", "Check QEMU processes")

    print("\n" + "=" * 60)
    print("🔍 DIAGNOSIS COMPLETE")

if __name__ == "__main__":
    main()