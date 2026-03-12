#!/usr/bin/env python3
"""
Simple VM connectivity check - minimal version to avoid timeout issues
"""

import socket
import os

def check_vm_connectivity():
    """Check if VMs are reachable on expected ports"""
    vm_ips = ["192.168.122.10", "192.168.122.11", "192.168.122.12", "192.168.122.13"]

    print("🔍 CHECKING VM CONNECTIVITY")
    print("-" * 40)

    for ip in vm_ips:
        print(f"Testing {ip}...")

        # Test if SSH port (22) is open
        try:
            sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            sock.settimeout(2)
            result = sock.connect_ex((ip, 22))
            sock.close()

            if result == 0:
                print(f"  ✅ SSH port open on {ip}")
            else:
                print(f"  ❌ SSH port closed on {ip}")

        except Exception as e:
            print(f"  ❌ Error connecting to {ip}: {e}")

    print("\nTo proceed with testing:")
    print("1. Start VMs via virt-manager (IPs 192.168.122.10-13)")
    print("2. Verify SSH keys are set up for amorphdb user")
    print("3. Check that amorphd services are running on VMs")

if __name__ == "__main__":
    check_vm_connectivity()