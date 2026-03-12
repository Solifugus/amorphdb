#!/bin/bash
"""
Setup SSH keys for AmorphDB distributed testing
Deploy keys to all 10 VMs using the amorphdb user (not root)
"""

# Configuration
KEY_NAME="amorphdb_cluster_key"
KEY_PATH="$HOME/.ssh/$KEY_NAME"
USERNAME="amorphdb"  # Using amorphdb user instead of root
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

echo -e "${GREEN}🔑 AmorphDB Cluster SSH Key Setup (Fixed)${NC}"
echo "=================================================="
echo -e "Using user: ${GREEN}$USERNAME${NC} (not root)"

# Step 1: Use existing SSH key
echo -e "\n${GREEN}🔧 Step 1: Using existing SSH key${NC}"
if [ -f "$KEY_PATH" ]; then
    echo -e "${GREEN}✅ SSH key exists: $KEY_PATH${NC}"
else
    echo -e "${RED}❌ SSH key not found: $KEY_PATH${NC}"
    exit 1
fi

# Step 2: Test connectivity to all VMs with amorphdb user
echo -e "\n${GREEN}🔧 Step 2: Testing VM connectivity (amorphdb user)${NC}"
REACHABLE_VMS=()

for i in "${!VM_IPS[@]}"; do
    ip="${VM_IPS[$i]}"
    vm_name="amorphdb$i"

    echo -n "Testing $vm_name ($ip) with user '$USERNAME'... "

    if timeout 10 sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5 "$USERNAME@$ip" "echo SSH_TEST_SUCCESS" 2>/dev/null | grep -q "SSH_TEST_SUCCESS"; then
        echo -e "${GREEN}✅ Working${NC}"
        REACHABLE_VMS+=("$ip")
    else
        echo -e "${RED}❌ Failed${NC}"
    fi
done

echo -e "\nReachable VMs with '$USERNAME' user: ${GREEN}${#REACHABLE_VMS[@]}/10${NC}"

if [ ${#REACHABLE_VMS[@]} -eq 0 ]; then
    echo -e "${RED}❌ No VMs reachable with user '$USERNAME'${NC}"
    exit 1
fi

# Step 3: Deploy SSH keys to reachable VMs
echo -e "\n${GREEN}🔧 Step 3: Deploying SSH keys to '$USERNAME' user${NC}"
SUCCESSFUL_DEPLOYMENTS=()

for ip in "${REACHABLE_VMS[@]}"; do
    vm_index=$((${ip##*.} - 10))
    vm_name="amorphdb$vm_index"

    echo -n "Deploying key to $vm_name ($ip)... "

    if sshpass -p "$PASSWORD" ssh-copy-id -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i "$KEY_PATH" "$USERNAME@$ip" >/dev/null 2>&1; then
        echo -e "${GREEN}✅ Success${NC}"
        SUCCESSFUL_DEPLOYMENTS+=("$ip")
    else
        echo -e "${RED}❌ Failed${NC}"
    fi
done

# Step 4: Test passwordless SSH connections
echo -e "\n${GREEN}🔧 Step 4: Testing passwordless SSH${NC}"
WORKING_CONNECTIONS=()

for ip in "${SUCCESSFUL_DEPLOYMENTS[@]}"; do
    vm_index=$((${ip##*.} - 10))
    vm_name="amorphdb$vm_index"

    echo -n "Testing passwordless SSH to $vm_name ($ip)... "

    if ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5 -i "$KEY_PATH" "$USERNAME@$ip" "echo 'SSH_SUCCESS'" 2>/dev/null | grep -q "SSH_SUCCESS"; then
        echo -e "${GREEN}✅ Working${NC}"
        WORKING_CONNECTIONS+=("$ip")
    else
        echo -e "${RED}❌ Failed${NC}"
    fi
done

# Step 5: Setup sudo access for AmorphDB installation
echo -e "\n${GREEN}🔧 Step 5: Configuring sudo access for AmorphDB${NC}"
SUDO_CONFIGURED=()

for ip in "${WORKING_CONNECTIONS[@]}"; do
    vm_index=$((${ip##*.} - 10))
    vm_name="amorphdb$vm_index"

    echo -n "Setting up sudo for $vm_name ($ip)... "

    # Test if amorphdb user already has sudo
    if ssh -i "$KEY_PATH" "$USERNAME@$ip" "sudo -n true" 2>/dev/null; then
        echo -e "${GREEN}✅ Already configured${NC}"
        SUDO_CONFIGURED+=("$ip")
    else
        echo -e "${YELLOW}⚠️  Needs configuration${NC}"
        # Note: In a real deployment, this would need to be configured manually
        # or the amorphdb user would need to be in the sudo group
    fi
done

# Generate connection summary and SSH config
echo -e "\n${GREEN}📊 SSH Key Deployment Results${NC}"
echo "=============================================="
echo -e "Total VMs: ${#VM_IPS[@]}"
echo -e "Reachable with '$USERNAME': ${GREEN}${#REACHABLE_VMS[@]}${NC}"
echo -e "SSH Keys Deployed: ${GREEN}${#SUCCESSFUL_DEPLOYMENTS[@]}${NC}"
echo -e "Working SSH Connections: ${GREEN}${#WORKING_CONNECTIONS[@]}${NC}"

# Create SSH config file for easier access
SSH_CONFIG="$HOME/.ssh/amorphdb_config"
echo -e "\n${GREEN}🚀 Creating SSH Configuration${NC}"

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

# Generate connection commands for Claude
echo -e "\n${GREEN}🚀 SSH Connections Ready for Claude${NC}"
echo "=============================================="
echo "# SSH Key: $KEY_PATH"
echo "# Username: $USERNAME (with sudo access)"
echo "# SSH Config: $SSH_CONFIG"
echo ""

if [ ${#WORKING_CONNECTIONS[@]} -gt 0 ]; then
    echo "# Working connections:"
    for ip in "${WORKING_CONNECTIONS[@]}"; do
        vm_index=$((${ip##*.} - 10))
        vm_name="amorphdb$vm_index"
        echo "# $vm_name: ssh -F $SSH_CONFIG $vm_name"
    done

    echo ""
    echo "# Quick test command:"
    echo "ssh -F $SSH_CONFIG amorphdb0 \"hostname && whoami && sudo whoami\""

    echo ""
    echo "# For Claude to use:"
    echo "ssh_key='$KEY_PATH'"
    echo "ssh_config='$SSH_CONFIG'"
    echo "username='$USERNAME'"
    echo "vm_ips=(${WORKING_CONNECTIONS[*]})"
fi

# Final summary
echo -e "\n${GREEN}🎯 Final Summary${NC}"
echo "=============================================="
if [ ${#WORKING_CONNECTIONS[@]} -ge 2 ]; then
    echo -e "${GREEN}✅ SUCCESS! Ready for distributed AmorphDB testing${NC}"
    echo "Working VMs: ${#WORKING_CONNECTIONS[@]}/10"
    echo ""
    echo "Claude can now:"
    echo "- Use fast SCP for binary deployment"
    echo "- Test distributed mesh with ${#WORKING_CONNECTIONS[@]} nodes"
    echo "- Validate real AmorphDB functionality"

    if [ ${#SUDO_CONFIGURED[@]} -lt ${#WORKING_CONNECTIONS[@]} ]; then
        echo ""
        echo -e "${YELLOW}⚠️  Note: Some VMs may need sudo configuration for full AmorphDB setup${NC}"
        echo "You may need to add '$USERNAME' to sudoers on VMs without sudo access"
    fi
else
    echo -e "${RED}❌ Insufficient working VMs for distributed testing${NC}"
fi

echo -e "\n${GREEN}🔑 Setup complete!${NC}"