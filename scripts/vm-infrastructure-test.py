#!/usr/bin/env python3
"""
AmorphDB VM Infrastructure Test
Test basic VM creation and Console Bridge connectivity
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

def test_vm_infrastructure():
    """Test basic VM creation and console connectivity"""
    print("🧪 Testing VM Infrastructure")
    print("=" * 40)

    base_image = "/home/solifugus/development/YakirOS/yakiros-vm.qcow2"
    test_socket = "/tmp/test-vm.sock"
    test_image = "/tmp/test-vm.qcow2"

    try:
        # 1. Test VM image creation
        print("🔧 Step 1: Creating test VM image...")
        cmd = [
            "qemu-img", "create", "-f", "qcow2",
            "-F", "qcow2", "-b", base_image,
            test_image, "2G"
        ]
        result = subprocess.run(cmd, capture_output=True, text=True)
        if result.returncode != 0:
            print(f"❌ VM image creation failed: {result.stderr}")
            return False
        print("✅ VM image created successfully")

        # 2. Test VM launch with console
        print("🚀 Step 2: Launching test VM...")
        cmd = [
            "qemu-system-x86_64",
            "-enable-kvm",
            "-m", "1G",
            "-smp", "1",
            "-drive", f"file={test_image},if=virtio",
            "-netdev", "user,id=net0",
            "-device", "virtio-net,netdev=net0",
            "-chardev", f"socket,id=console,path={test_socket},server=on,wait=off",
            "-serial", "chardev:console",
            "-display", "none",
            "-daemonize",
            "-pidfile", "/tmp/test-vm.pid"
        ]

        result = subprocess.run(cmd, capture_output=True, text=True)
        if result.returncode != 0:
            print(f"❌ VM launch failed: {result.stderr}")
            return False
        print("✅ VM launched successfully")

        # 3. Wait for socket
        print("⏳ Step 3: Waiting for console socket...")
        for i in range(30):
            if os.path.exists(test_socket):
                print("✅ Console socket ready")
                break
            time.sleep(1)
        else:
            print("❌ Console socket not ready after 30 seconds")
            return False

        # 4. Test Console Bridge connection
        print("🔌 Step 4: Testing Console Bridge connection...")
        try:
            with QEMUConsole(test_socket, timeout=30) as console:
                # Wait for any boot output
                time.sleep(5)

                # Try to read any available output
                output = console.read_output()

                # Handle both string and list output
                if isinstance(output, list):
                    output_str = '\n'.join(output)
                else:
                    output_str = str(output)

                print(f"📋 Console output (first 200 chars): {output_str[:200]}...")

                # Try basic command if we can
                if "login:" in output_str.lower():
                    print("✅ Console Bridge connection working - login prompt detected")
                elif len(output_str) > 0:
                    print("✅ Console Bridge connection working - boot output received")
                else:
                    print("⚠️  Console Bridge connected but no output yet")

                return True

        except Exception as e:
            print(f"❌ Console Bridge connection failed: {e}")
            return False

    except Exception as e:
        print(f"❌ Infrastructure test failed: {e}")
        return False

    finally:
        # Cleanup
        print("🧹 Cleaning up test VM...")
        try:
            if os.path.exists("/tmp/test-vm.pid"):
                with open("/tmp/test-vm.pid") as f:
                    pid = f.read().strip()
                subprocess.run(["kill", pid], check=False)
                os.unlink("/tmp/test-vm.pid")
        except:
            pass

        for file_path in [test_socket, test_image]:
            try:
                if os.path.exists(file_path):
                    os.unlink(file_path)
            except:
                pass

if __name__ == "__main__":
    print("🎯 AmorphDB VM Infrastructure Test")
    print("Testing QEMU + Console Bridge Foundation")
    print("=" * 50)

    # Check prerequisites
    prereqs = [
        "/home/solifugus/development/YakirOS/yakiros-vm.qcow2",
        "/home/solifugus/development/qemu-console-bridge/src/qemu_console.py"
    ]

    for prereq in prereqs:
        if not os.path.exists(prereq):
            print(f"❌ Missing prerequisite: {prereq}")
            sys.exit(1)

    print("✅ All prerequisites found")

    if test_vm_infrastructure():
        print("\n🎉 INFRASTRUCTURE TEST PASSED!")
        print("✅ Ready for full AmorphDB distributed testing")
        sys.exit(0)
    else:
        print("\n❌ INFRASTRUCTURE TEST FAILED")
        print("Fix infrastructure issues before running full tests")
        sys.exit(1)
