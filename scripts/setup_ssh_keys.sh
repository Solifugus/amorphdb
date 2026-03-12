#!/bin/bash
"""
Setup SSH keys for AmorphDB distributed testing
Deploy keys to all 10 VMs with static IPs
"""

# Configuration
KEY_NAME="amorphdb_cluster_key"
KEY_PATH="$HOME/.ssh/$KEY_NAME"
USERNAME="root"
PASSWORD="amorphdb123"
DOMAIN="amorphdb"

# VM IP range
VM_IPS=(
    "192.168.122.10"  # amorphdb0
    "192.168.122.11"  # amorphdb1
    "192.168.122.12"  # amorphdb2
    "192.168.122.13"  # amorphdb3
    "192.168.122.14"  # amorphdb4
    "192.168.122.15"  # amorphdb5
    "192.168.122.16"  # amorphdb6
    "192.168.122.17"  # amorphdb7
    "192.168.122.18"  # amorphdb8
    "192.168.122.19"  # amorphdb9
)

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🔑 AmorphDB Cluster SSH Key Setup${NC}"
echo "=================================================="

# Check if sshpass is available (needed for password-based key deployment)
if ! command -v sshpass &> /dev/null; then
    echo -e "${YELLOW}⚠️  sshpass not found. Installing...${NC}"
    if command -v apt &> /dev/null; then
        sudo apt update && sudo apt install -y sshpass
    elif command -v yum &> /dev/null; then
        sudo yum install -y sshpass
    elif command -v pacman &> /dev/null; then
        sudo pacman -S sshpass
    else
        echo -e "${RED}❌ Cannot install sshpass automatically. Please install it manually.${NC}"
        exit 1
    fi
fi

# Step 1: Generate SSH key if it doesn't exist
echo -e "\n${GREEN}🔧 Step 1: Generating SSH key${NC}"
if [ ! -f "$KEY_PATH" ]; then
    ssh-keygen -t ed25519 -f "$KEY_PATH" -N "" -C "amorphdb-cluster-$(date +%Y%m%d)"
    echo -e "${GREEN}✅ SSH key generated: $KEY_PATH${NC}"
else
    echo -e "${YELLOW}⚠️  SSH key already exists: $KEY_PATH${NC}"
    read -p "Overwrite existing key? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -f "$KEY_PATH" "$KEY_PATH.pub"
        ssh-keygen -t ed25519 -f "$KEY_PATH" -N "" -C "amorphdb-cluster-$(date +%Y%m%d)"
        echo -e "${GREEN}✅ New SSH key generated${NC}"
    fi
fi

# Step 2: Test connectivity to all VMs
echo -e "\n${GREEN}🔧 Step 2: Testing VM connectivity${NC}"
REACHABLE_VMS=()
UNREACHABLE_VMS=()

for i in "${!VM_IPS[@]}"; do
    ip="${VM_IPS[$i]}"
    vm_name="amorphdb$i"

    echo -n "Testing $vm_name ($ip)... "

    if timeout 5 bash -c "</dev/tcp/$ip/22" 2>/dev/null; then
        echo -e "${GREEN}✅ Reachable${NC}"
        REACHABLE_VMS+=("$ip")
    else
        echo -e "${RED}❌ Unreachable${NC}"
        UNREACHABLE_VMS+=("$ip")
    fi
done

if [ ${#UNREACHABLE_VMS[@]} -gt 0 ]; then
    echo -e "\n${YELLOW}⚠️  Unreachable VMs: ${UNREACHABLE_VMS[*]}${NC}"
    echo "Please ensure these VMs are running and SSH is enabled."
    read -p "Continue with reachable VMs only? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Step 3: Deploy SSH keys to reachable VMs
echo -e "\n${GREEN}🔧 Step 3: Deploying SSH keys${NC}"
SUCCESSFUL_DEPLOYMENTS=()
FAILED_DEPLOYMENTS=()

for ip in "${REACHABLE_VMS[@]}"; do
    vm_index=$((${ip##*.} - 10))  # Extract last octet and convert to VM index
    vm_name="amorphdb$vm_index"

    echo -n "Deploying key to $vm_name ($ip)... "

    # Copy SSH key using sshpass
    if sshpass -p "$PASSWORD" ssh-copy-id -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i "$KEY_PATH" "$USERNAME@$ip" >/dev/null 2>&1; then
        echo -e "${GREEN}✅ Success${NC}"
        SUCCESSFUL_DEPLOYMENTS+=("$ip")
    else
        echo -e "${RED}❌ Failed${NC}"
        FAILED_DEPLOYMENTS+=("$ip")
    fi
done

# Step 4: Test passwordless SSH connections
echo -e "\n${GREEN}🔧 Step 4: Testing passwordless SSH${NC}"
WORKING_CONNECTIONS=()
FAILED_CONNECTIONS=()

for ip in "${SUCCESSFUL_DEPLOYMENTS[@]}"; do
    vm_index=$((${ip##*.} - 10))
    vm_name="amorphdb$vm_index"

    echo -n "Testing passwordless SSH to $vm_name ($ip)... "

    if ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5 -i "$KEY_PATH" "$USERNAME@$ip" "echo 'SSH_SUCCESS'" 2>/dev/null | grep -q "SSH_SUCCESS"; then
        echo -e "${GREEN}✅ Working${NC}"
        WORKING_CONNECTIONS+=("$ip")
    else
        echo -e "${RED}❌ Failed${NC}"
        FAILED_CONNECTIONS+=("$ip")
    fi
done

# Step 5: Generate connection summary
echo -e "\n${GREEN}📊 SSH Key Deployment Results${NC}"
echo "=============================================="
echo -e "Total VMs: ${#VM_IPS[@]}"
echo -e "Reachable: ${GREEN}${#REACHABLE_VMS[@]}${NC}"
echo -e "SSH Keys Deployed: ${GREEN}${#SUCCESSFUL_DEPLOYMENTS[@]}${NC}"
echo -e "Working SSH Connections: ${GREEN}${#WORKING_CONNECTIONS[@]}${NC}"

if [ ${#FAILED_DEPLOYMENTS[@]} -gt 0 ]; then
    echo -e "Failed Deployments: ${RED}${#FAILED_DEPLOYMENTS[@]}${NC}"
fi

if [ ${#FAILED_CONNECTIONS[@]} -gt 0 ]; then
    echo -e "Failed Connections: ${RED}${#FAILED_CONNECTIONS[@]}${NC}"
fi

# Generate connection commands for Claude
echo -e "\n${GREEN}🚀 SSH Connection Commands for Claude${NC}"
echo "=============================================="
echo "# SSH Key Path: $KEY_PATH"
echo "# Username: $USERNAME"
echo "# Password (if needed): $PASSWORD"
echo ""

if [ ${#WORKING_CONNECTIONS[@]} -gt 0 ]; then
    echo "# Working SSH connections:"
    for ip in "${WORKING_CONNECTIONS[@]}"; do
        vm_index=$((${ip##*.} - 10))
        vm_name="amorphdb$vm_index"

        echo "# $vm_name: ssh -i $KEY_PATH $USERNAME@$ip"
    done

    echo ""
    echo "# Test all connections:"
    echo "for ip in ${WORKING_CONNECTIONS[*]}; do"
    echo "    echo \"Testing \$ip...\""
    echo "    ssh -i $KEY_PATH $USERNAME@\$ip \"hostname && uptime\""
    echo "done"

    # Create SSH config file for easier access
    SSH_CONFIG="$HOME/.ssh/amorphdb_config"
    echo ""
    echo "# Creating SSH config file: $SSH_CONFIG"

    cat > "$SSH_CONFIG" << EOF
# AmorphDB Cluster SSH Configuration
# Usage: ssh -F $SSH_CONFIG amorphdb0

Host amorphdb*
    User $USERNAME
    IdentityFile $KEY_PATH
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null

EOF

    # Add each working VM to SSH config
    for ip in "${WORKING_CONNECTIONS[@]}"; do
        vm_index=$((${ip##*.} - 10))
        vm_name="amorphdb$vm_index"

        cat >> "$SSH_CONFIG" << EOF
Host $vm_name
    HostName $ip

EOF
    done

    echo -e "${GREEN}✅ SSH config created: $SSH_CONFIG${NC}"
    echo ""
    echo "# Quick connection test using SSH config:"
    echo "ssh -F $SSH_CONFIG amorphdb0 \"echo 'Connected to amorphdb0'\""

else
    echo -e "${RED}❌ No working SSH connections established${NC}"
    echo "Please check VM status and SSH configuration."
fi

# Final summary for Claude
echo -e "\n${GREEN}🎯 Summary for Claude Distributed Testing${NC}"
echo "=============================================="
if [ ${#WORKING_CONNECTIONS[@]} -ge 2 ]; then
    echo -e "${GREEN}✅ Ready for distributed AmorphDB testing!${NC}"
    echo "Working VMs: ${#WORKING_CONNECTIONS[@]}/10"
    echo "SSH Key: $KEY_PATH"
    echo "SSH Config: $SSH_CONFIG"
    echo ""
    echo "Claude can now use these VMs for:"
    echo "- Fast SCP binary deployment"
    echo "- Distributed mesh testing"
    echo "- Multi-node AmorphDB validation"
elif [ ${#WORKING_CONNECTIONS[@]} -eq 1 ]; then
    echo -e "${YELLOW}⚠️  Only 1 working VM - limited testing possible${NC}"
else
    echo -e "${RED}❌ No working VMs - troubleshooting needed${NC}"
fi

echo ""
echo -e "${GREEN}🔑 Setup complete!${NC}"