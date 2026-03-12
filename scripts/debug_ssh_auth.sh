#!/bin/bash

# Quick SSH authentication debug
USERNAME="root"
PASSWORD="amorphdb123"
TEST_VM="192.168.122.10"

echo "🔍 SSH Authentication Debug"
echo "==========================="

echo "Testing VM: $TEST_VM"
echo "Username: $USERNAME"
echo "Password: $PASSWORD"
echo ""

# Test 1: Basic SSH connection with password
echo "🧪 Test 1: Basic SSH with password"
echo -n "Testing: sshpass -p '$PASSWORD' ssh $USERNAME@$TEST_VM 'echo SUCCESS'... "

if timeout 10 sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5 "$USERNAME@$TEST_VM" "echo SUCCESS" 2>/dev/null | grep -q "SUCCESS"; then
    echo "✅ SUCCESS"
    SSH_BASIC_WORKS=true
else
    echo "❌ FAILED"
    SSH_BASIC_WORKS=false
fi

# Test 2: Try with amorphdb user instead of root
echo ""
echo "🧪 Test 2: Basic SSH with amorphdb user"
USERNAME_ALT="amorphdb"
echo -n "Testing: sshpass -p '$PASSWORD' ssh $USERNAME_ALT@$TEST_VM 'echo SUCCESS'... "

if timeout 10 sshpass -p "$PASSWORD" ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5 "$USERNAME_ALT@$TEST_VM" "echo SUCCESS" 2>/dev/null | grep -q "SUCCESS"; then
    echo "✅ SUCCESS"
    SSH_ALT_WORKS=true
else
    echo "❌ FAILED"
    SSH_ALT_WORKS=false
fi

# Test 3: Check what's happening with verbose output
echo ""
echo "🧪 Test 3: Verbose SSH output (for debugging)"
echo "Running: ssh -v $USERNAME@$TEST_VM"
echo "Expected to see authentication methods..."
echo ""

timeout 10 ssh -v -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$USERNAME@$TEST_VM" "echo TEST" 2>&1 | head -20

# Summary and recommendations
echo ""
echo "🎯 Debug Summary"
echo "================"

if [ "$SSH_BASIC_WORKS" = true ]; then
    echo "✅ Root SSH working - script should have worked"
    echo "Recommendation: Try running setup script again"
elif [ "$SSH_ALT_WORKS" = true ]; then
    echo "✅ amorphdb user SSH working, root disabled"
    echo "Recommendation: Modify script to use 'amorphdb' user instead"
else
    echo "❌ Neither root nor amorphdb SSH working"
    echo "Possible issues:"
    echo "  - Wrong password"
    echo "  - SSH not configured for password auth"
    echo "  - PermitRootLogin disabled"
    echo "  - SSH service not properly configured"
    echo ""
    echo "Next steps:"
    echo "  1. Check SSH config on one VM manually"
    echo "  2. Verify passwords are correct"
    echo "  3. Check if SSH allows password authentication"
fi