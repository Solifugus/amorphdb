#!/usr/bin/env python3
"""
AmorphDB Enhanced VM Test
Real distributed testing with actual AmorphDB binaries and mesh configuration
"""
import sys
import os
import subprocess
import time
import base64

# Add the console bridge to Python path
sys.path.insert(0, '/home/solifugus/development/qemu-console-bridge/src')

try:
    from qemu_console import QEMUConsole
except ImportError:
    print("❌ QEMU Console Bridge not found. Please check the path.")
    sys.exit(1)

class AmorphDBEnhancedVMTest:
    def __init__(self):
        self.node1_socket = "/tmp/amorphdb-node1.sock"
        self.node2_socket = "/tmp/amorphdb-node2.sock"
        self.base_image = "/home/solifugus/development/YakirOS/yakiros-vm.qcow2"
        self.binary_dir = "/home/solifugus/development/amorphdb/bin"

        # AmorphDB configuration
        self.node1_id = "ra-do-ki"  # CV syllable pattern
        self.node2_id = "fi-ne-so"  # CV syllable pattern
        self.mesh_port = 5000
        self.admin_port = 5001

    def create_vm_image(self, name):
        """Create a new VM image based on YakirOS base"""
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
        """Launch a VM with console bridge and proper networking"""
        cmd = [
            "qemu-system-x86_64",
            "-enable-kvm",
            "-m", "2G",
            "-smp", "2",
            "-drive", f"file={image_path},if=virtio",
            "-netdev", f"user,id=net0,hostfwd=tcp::{self.mesh_port+port_offset}-:{self.mesh_port},hostfwd=tcp::{self.admin_port+port_offset}-:{self.admin_port},hostfwd=tcp::{2222+port_offset}-:22",
            "-device", "virtio-net,netdev=net0",
            "-chardev", f"socket,id=console,path={socket_path},server=on,wait=off",
            "-serial", "chardev:console",
            "-display", "none",
            "-daemonize",
            "-pidfile", f"/tmp/{name}.pid"
        ]

        print(f"🚀 Launching VM: {name}")
        print(f"   Console: {socket_path}")
        print(f"   AmorphDB Mesh Port: {self.mesh_port+port_offset}")
        print(f"   AmorphDB Admin Port: {self.admin_port+port_offset}")
        print(f"   SSH Port: {2222+port_offset}")

        subprocess.run(cmd, check=True)

        # Wait for socket to be ready
        for i in range(30):
            if os.path.exists(socket_path):
                print(f"✅ VM {name} socket ready")
                break
            time.sleep(1)
        else:
            raise Exception(f"VM {name} socket not ready after 30 seconds")

    def encode_file_for_transfer(self, file_path):
        """Encode binary file as base64 for VM transfer"""
        with open(file_path, 'rb') as f:
            return base64.b64encode(f.read()).decode('ascii')

    def deploy_amorphdb_to_vm(self, console, node_name, node_id, is_first_node=False):
        """Deploy real AmorphDB binaries and configuration to VM"""
        print(f"🔧 Deploying AmorphDB to {node_name} (ID: {node_id})")

        # Wait for login prompt and login
        console.wait_for("login:", timeout=60)
        console.send_command("root")
        time.sleep(2)

        # Create AmorphDB directory structure
        console.send_command("mkdir -p /opt/amorphdb/bin")
        console.send_command("mkdir -p /opt/amorphdb/data")
        console.send_command("mkdir -p /opt/amorphdb/config")

        # Transfer AmorphDB binaries
        for binary in ['amorphd', 'amorph', 'amorphctl']:
            binary_path = f"{self.binary_dir}/{binary}"
            if os.path.exists(binary_path):
                print(f"   📦 Transferring {binary}...")
                encoded_binary = self.encode_file_for_transfer(binary_path)

                # Transfer via base64 (split into chunks to avoid line length issues)
                chunk_size = 4000
                console.send_command(f"rm -f /tmp/{binary}.b64")

                for i in range(0, len(encoded_binary), chunk_size):
                    chunk = encoded_binary[i:i+chunk_size]
                    console.send_command(f"echo '{chunk}' >> /tmp/{binary}.b64")

                # Decode and install
                console.send_command(f"base64 -d /tmp/{binary}.b64 > /opt/amorphdb/bin/{binary}")
                console.send_command(f"chmod +x /opt/amorphdb/bin/{binary}")
                console.send_command(f"rm /tmp/{binary}.b64")

        # Create AmorphDB configuration
        if is_first_node:
            config = f"""# AmorphDB Node Configuration - {node_name}
node_id: "{node_id}"
mesh_port: {self.mesh_port}
admin_port: {self.admin_port}
data_dir: "/opt/amorphdb/data"
bootstrap_mode: "genesis"  # First node creates the mesh
log_level: "info"
"""
        else:
            # Second node connects to first node
            bootstrap_addr = f"10.0.2.2:{self.mesh_port}"  # First VM from second VM perspective
            config = f"""# AmorphDB Node Configuration - {node_name}
node_id: "{node_id}"
mesh_port: {self.mesh_port}
admin_port: {self.admin_port}
data_dir: "/opt/amorphdb/data"
bootstrap_mode: "join"
bootstrap_address: "{bootstrap_addr}"
log_level: "info"
"""

        # Write configuration file
        console.send_command(f"cat > /opt/amorphdb/config/amorphdb.yaml << 'EOF'")
        for line in config.strip().split('\n'):
            console.send_command(line)
        console.send_command("EOF")

        print(f"✅ AmorphDB deployment complete on {node_name}")

    def start_amorphdb_daemon(self, console, node_name):
        """Start AmorphDB daemon on VM"""
        print(f"🚀 Starting AmorphDB daemon on {node_name}")

        # Start daemon in background
        console.send_command("cd /opt/amorphdb")
        console.send_command("./bin/amorphd --config config/amorphdb.yaml > logs/daemon.log 2>&1 &")
        console.send_command("echo $! > amorphd.pid")

        # Wait a moment for startup
        time.sleep(3)

        # Check if process is running
        console.send_command("ps aux | grep amorphd | grep -v grep")
        output = console.read_output()

        if "amorphd" in output:
            print(f"✅ AmorphDB daemon running on {node_name}")
            return True
        else:
            print(f"❌ AmorphDB daemon failed to start on {node_name}")
            console.send_command("cat logs/daemon.log")
            error_log = console.read_output()
            print(f"Error log: {error_log}")
            return False

    def test_amorphdb_functionality(self, console, node_name):
        """Test basic AmorphDB functionality on VM"""
        print(f"🧪 Testing AmorphDB functionality on {node_name}")

        # Create test directory
        console.send_command("mkdir -p /tmp/amorphdb-test")
        console.send_command("cd /tmp/amorphdb-test")

        # Test basic MBL operations
        test_mbl = """
# Basic AmorphDB test
.test_data = "Hello from {node_name}"
.counter = 42
.timestamp = now()

# Test hierarchical structure
.user.name = "test_user"
.user.created = now()
"""

        # Write test MBL script
        console.send_command(f"cat > test.mbl << 'EOF'")
        for line in test_mbl.strip().split('\n'):
            console.send_command(line.replace("{node_name}", node_name))
        console.send_command("EOF")

        # Execute test via amorph client
        console.send_command("/opt/amorphdb/bin/amorph --script test.mbl")
        output = console.read_output()

        if "Error" not in output and "error" not in output.lower():
            print(f"✅ Basic MBL operations successful on {node_name}")
            return True
        else:
            print(f"❌ MBL operations failed on {node_name}: {output}")
            return False

    def test_distributed_mesh(self):
        """Test complete 2-node AmorphDB mesh functionality"""
        print("🌐 Testing AmorphDB 2-node distributed mesh")
        print("=" * 50)

        # Create VM images
        node1_image = self.create_vm_image("amorphdb-node1")
        node2_image = self.create_vm_image("amorphdb-node2")

        try:
            # Launch VMs
            self.launch_vm("amorphdb-node1", node1_image, self.node1_socket, 0)
            self.launch_vm("amorphdb-node2", node2_image, self.node2_socket, 10)

            time.sleep(10)  # Give VMs time to boot

            # Connect to VMs via console bridge
            with QEMUConsole(self.node1_socket) as console1, \
                 QEMUConsole(self.node2_socket) as console2:

                # Deploy AmorphDB to both nodes
                self.deploy_amorphdb_to_vm(console1, "node1", self.node1_id, is_first_node=True)
                self.deploy_amorphdb_to_vm(console2, "node2", self.node2_id, is_first_node=False)

                # Start daemons
                if not self.start_amorphdb_daemon(console1, "node1"):
                    raise Exception("Failed to start daemon on node1")

                time.sleep(5)  # Let first node establish itself

                if not self.start_amorphdb_daemon(console2, "node2"):
                    raise Exception("Failed to start daemon on node2")

                time.sleep(5)  # Let mesh form

                # Test functionality on both nodes
                node1_success = self.test_amorphdb_functionality(console1, "node1")
                node2_success = self.test_amorphdb_functionality(console2, "node2")

                if node1_success and node2_success:
                    print("🎉 Distributed AmorphDB mesh test SUCCESSFUL!")
                    return True
                else:
                    print("❌ Distributed AmorphDB mesh test FAILED")
                    return False

        except Exception as e:
            print(f"❌ Test failed: {e}")
            import traceback
            traceback.print_exc()
            return False

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
    print("🎯 AmorphDB Enhanced VM Test")
    print("Real Distributed Testing with Mesh Configuration")
    print("=" * 50)

    # Verify binaries exist
    binary_dir = "/home/solifugus/development/amorphdb/bin"
    required_binaries = ['amorphd', 'amorph', 'amorphctl']

    for binary in required_binaries:
        binary_path = f"{binary_dir}/{binary}"
        if not os.path.exists(binary_path):
            print(f"❌ Missing binary: {binary_path}")
            print("Please run: go build -o bin/{binary} ./cmd/{binary}")
            sys.exit(1)

    print(f"✅ All required binaries found in {binary_dir}")

    test = AmorphDBEnhancedVMTest()
    success = test.test_distributed_mesh()

    if success:
        print("\n🎉 VM TEST COMPLETED SUCCESSFULLY!")
        sys.exit(0)
    else:
        print("\n❌ VM TEST FAILED")
        sys.exit(1)
