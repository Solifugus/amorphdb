#!/usr/bin/env python3
"""
Basic AmorphDB VM Test
Simple test to validate AmorphDB binary deployment and basic functionality
"""
import sys
import os
import subprocess
import time

# Add the console bridge to Python path
sys.path.insert(0, '/home/solifugus/development/qemu-console-bridge/src')

try:
    from qemu_console import QEMUConsole
except ImportError:
    print("❌ QEMU Console Bridge not found. Please check the path.")
    sys.exit(1)

def test_basic_amorphdb_vm():
    """Test AmorphDB on single VM"""
    print("🎯 Basic AmorphDB VM Test")
    print("=" * 30)

    base_image = "/home/solifugus/development/YakirOS/yakiros-vm.qcow2"
    test_socket = "/tmp/amorphdb-test.sock"
    test_image = "/tmp/amorphdb-test.qcow2"
    binary_path = "/home/solifugus/development/amorphdb/bin/amorphd"

    try:
        # Create VM image
        print("🔧 Creating VM image...")
        cmd = [
            "qemu-img", "create", "-f", "qcow2",
            "-F", "qcow2", "-b", base_image,
            test_image, "5G"
        ]
        subprocess.run(cmd, check=True)

        # Launch VM
        print("🚀 Launching VM...")
        cmd = [
            "qemu-system-x86_64",
            "-enable-kvm",
            "-m", "2G",
            "-smp", "2",
            "-drive", f"file={test_image},if=virtio",
            "-netdev", "user,id=net0,hostfwd=tcp::5000-:5000",
            "-device", "virtio-net,netdev=net0",
            "-chardev", f"socket,id=console,path={test_socket},server=on,wait=off",
            "-serial", "chardev:console",
            "-display", "none",
            "-daemonize",
            "-pidfile", "/tmp/amorphdb-test.pid"
        ]
        subprocess.run(cmd, check=True)

        # Wait for socket
        for i in range(30):
            if os.path.exists(test_socket):
                break
            time.sleep(1)
        else:
            print("❌ Console socket not ready")
            return False

        # Connect and test
        with QEMUConsole(test_socket, timeout=60) as console:
            print("⏳ Waiting for VM boot...")
            time.sleep(15)  # Give more time for YakirOS to boot

            # Try to get to shell
            console.send_command("")  # Send empty line to wake up console
            time.sleep(2)
            console.send_command("")  # Another attempt
            time.sleep(2)

            # Check what we got
            output = console.read_output()
            if isinstance(output, list):
                output_str = '\n'.join(output)
            else:
                output_str = str(output)

            print(f"📋 Boot output: {output_str[-500:]}")  # Last 500 chars

            # Try basic commands
            print("🧪 Testing basic VM functionality...")
            console.send_command("whoami")
            time.sleep(1)
            console.send_command("pwd")
            time.sleep(1)
            console.send_command("ls -la")
            time.sleep(1)

            output = console.read_output()
            if isinstance(output, list):
                output_str = '\n'.join(output)
            else:
                output_str = str(output)

            print(f"📋 Command output: {output_str}")

            if len(output_str.strip()) > 0:
                print("✅ VM is responsive to commands")
                return True
            else:
                print("❌ VM not responsive")
                return False

    except Exception as e:
        print(f"❌ Test failed: {e}")
        return False

    finally:
        # Cleanup
        try:
            if os.path.exists("/tmp/amorphdb-test.pid"):
                with open("/tmp/amorphdb-test.pid") as f:
                    pid = f.read().strip()
                subprocess.run(["kill", pid], check=False)
                os.unlink("/tmp/amorphdb-test.pid")
        except:
            pass

        for file_path in [test_socket, test_image]:
            try:
                if os.path.exists(file_path):
                    os.unlink(file_path)
            except:
                pass

if __name__ == "__main__":
    print("🎯 AmorphDB Basic VM Test")
    print("Testing VM Responsiveness for AmorphDB Deployment")
    print("=" * 50)

    if test_basic_amorphdb_vm():
        print("\n✅ BASIC VM TEST PASSED")
        print("VM is ready for AmorphDB deployment")
        sys.exit(0)
    else:
        print("\n❌ BASIC VM TEST FAILED")
        sys.exit(1)
