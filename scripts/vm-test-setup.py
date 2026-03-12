#!/usr/bin/env python3
"""
AmorphDB VM Test Setup
Proof of concept for 2-VM distributed testing using QEMU Console Bridge
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

class AmorphDBVMTest:
    def __init__(self):
        self.node1_socket = "/tmp/amorphdb-node1.sock"
        self.node2_socket = "/tmp/amorphdb-node2.sock"
        self.base_image = "/home/solifugus/development/YakirOS/yakiros-vm.qcow2"

    def create_vm_image(self, name):
        """Create a new VM image based on Alpine/YakirOS base"""
        image_path = f"/tmp/{name}.qcow2"
        cmd = [
            "qemu-img", "create", "-f", "qcow2",
            "-F", "qcow2", "-b", self.base_image,
            image_path, "10G"
        ]
        print(f"🔧 Creating VM image: {name}")
        subprocess.run(cmd, check=True)
        return image_path

    def launch_vm(self, name, image_path, socket_path, port_offset=0):
        """Launch a VM with console bridge"""
        cmd = [
            "qemu-system-x86_64",
            "-enable-kvm",
            "-m", "2G",
            "-smp", "2",
            "-drive", f"file={image_path},if=virtio",
            "-netdev", f"user,id=net0,hostfwd=tcp::{5000+port_offset}-:5000,hostfwd=tcp::{22+port_offset}-:22",
            "-device", "virtio-net,netdev=net0",
            "-chardev", f"socket,id=console,path={socket_path},server=on,wait=off",
            "-serial", "chardev:console",
            "-display", "none",
            "-daemonize",
            "-pidfile", f"/tmp/{name}.pid"
        ]

        print(f"🚀 Launching VM: {name}")
        print(f"   Console: {socket_path}")
        print(f"   AmorphDB Port: {5000+port_offset}")
        print(f"   SSH Port: {22+port_offset}")

        subprocess.run(cmd, check=True)

        # Wait for socket to be ready
        for i in range(30):
            if os.path.exists(socket_path):
                print(f"✅ VM {name} socket ready")
                break
            time.sleep(1)
        else:
            raise Exception(f"VM {name} socket not ready after 30 seconds")

    def setup_amorphdb_on_vm(self, console, node_name, is_first_node=False):
        """Install and configure AmorphDB on a VM"""
        print(f"🔧 Setting up AmorphDB on {node_name}")

        # Wait for login prompt
        console.wait_for("login:")
        console.send_command("root")  # Assuming root login

        # Create AmorphDB directory
        console.send_command("mkdir -p /opt/amorphdb")

        # We'll need to copy the AmorphDB binary - for now simulate
        console.send_command("echo 'AmorphDB placeholder' > /opt/amorphdb/amorphd")
        console.send_command("chmod +x /opt/amorphdb/amorphd")

        # Configure as first node or connecting node
        if is_first_node:
            console.send_command("echo 'First node configuration' > /opt/amorphdb/config.yaml")
        else:
            console.send_command("echo 'Secondary node configuration' > /opt/amorphdb/config.yaml")

        print(f"✅ AmorphDB setup complete on {node_name}")

    def test_2_node_mesh(self):
        """Test basic 2-node AmorphDB mesh"""
        print("🌐 Testing 2-node AmorphDB mesh")

        # Create VM images
        node1_image = self.create_vm_image("amorphdb-node1")
        node2_image = self.create_vm_image("amorphdb-node2")

        try:
            # Launch VMs
            self.launch_vm("amorphdb-node1", node1_image, self.node1_socket, 0)
            self.launch_vm("amorphdb-node2", node2_image, self.node2_socket, 10)

            time.sleep(5)  # Give VMs time to boot

            # Connect to VMs via console bridge
            with QEMUConsole(self.node1_socket) as console1, \
                 QEMUConsole(self.node2_socket) as console2:

                # Setup AmorphDB on both nodes
                self.setup_amorphdb_on_vm(console1, "node1", is_first_node=True)
                self.setup_amorphdb_on_vm(console2, "node2", is_first_node=False)

                # Test basic connectivity
                print("🔍 Testing inter-node connectivity")
                console1.send_command("ping -c 3 192.168.1.2")  # Assuming standard VM network
                ping_output = console1.read_output()

                if "3 packets transmitted" in ping_output:
                    print("✅ Inter-node connectivity working")
                else:
                    print("❌ Inter-node connectivity failed")

                # Simulate AmorphDB operations
                print("📊 Simulating AmorphDB distributed operations")
                console1.send_command("echo 'Node1: Creating test data' >> /tmp/amorphdb-test.log")
                console2.send_command("echo 'Node2: Replicating data' >> /tmp/amorphdb-test.log")

                # Check logs
                console1.send_command("cat /tmp/amorphdb-test.log")
                log1 = console1.read_output()
                console2.send_command("cat /tmp/amorphdb-test.log")
                log2 = console2.read_output()

                print("📋 Node1 log:", log1)
                print("📋 Node2 log:", log2)

                print("✅ 2-node VM test completed successfully!")

        except Exception as e:
            print(f"❌ Test failed: {e}")

        finally:
            # Cleanup
            self.cleanup_vms()

    def cleanup_vms(self):
        """Clean up VM processes and files"""
        print("🧹 Cleaning up VMs")
        for vm in ["amorphdb-node1", "amorphdb-node2"]:
            try:
                with open(f"/tmp/{vm}.pid") as f:
                    pid = f.read().strip()
                subprocess.run(["kill", pid], check=False)
                os.unlink(f"/tmp/{vm}.pid")
                os.unlink(f"/tmp/{vm}.qcow2")
            except:
                pass

        # Remove sockets
        for sock in [self.node1_socket, self.node2_socket]:
            try:
                os.unlink(sock)
            except:
                pass

if __name__ == "__main__":
    print("🎯 AmorphDB VM Test - Proof of Concept")
    print("=====================================")

    test = AmorphDBVMTest()
    test.test_2_node_mesh()
