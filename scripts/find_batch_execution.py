#!/usr/bin/env python3
"""
Find Correct Batch Execution Method for AmorphDB
Check command line options and execution modes
"""

import subprocess

def check_amorph_help():
    """Check amorph command line options"""
    print("🔍 CHECKING AMORPH COMMAND LINE OPTIONS")
    print("=" * 50)

    ip = "192.168.122.10"

    # Get full help output
    help_command = "cd /home/amorphdb && ./amorph --help"

    help_result = subprocess.run([
        "ssh", "-o", "ConnectTimeout=5", f"amorphdb@{ip}", help_command
    ], capture_output=True, text=True, timeout=10)

    if help_result.stdout:
        print("📖 AmorphDB Help Output:")
        print("-" * 30)
        print(help_result.stdout)
        print("-" * 30)

    if help_result.stderr:
        print("⚠️ Help Errors:")
        print(help_result.stderr)

    return help_result.returncode == 0

def test_batch_flags():
    """Test potential batch execution flags"""
    print(f"\n🧪 TESTING POTENTIAL BATCH FLAGS")
    print("=" * 50)

    ip = "192.168.122.10"

    # Test various flags that might enable batch mode
    flags_to_test = [
        "-batch",
        "--batch",
        "-script",
        "--script",
        "-exec",
        "--exec",
        "-run",
        "--run",
        "-file",
        "--file",
        "-non-interactive",
        "--non-interactive"
    ]

    for flag in flags_to_test:
        print(f"🔍 Testing {flag}...")

        test_command = f"cd /home/amorphdb && ./amorph {flag} 2>&1 | head -3"

        test_result = subprocess.run([
            "ssh", "-o", "ConnectTimeout=5", f"amorphdb@{ip}", test_command
        ], capture_output=True, text=True, timeout=8)

        if test_result.stdout and "Usage:" not in test_result.stdout and "unknown" not in test_result.stdout.lower():
            print(f"   ✅ {flag} recognized!")
            print(f"   Output: {test_result.stdout.strip()}")
        else:
            print(f"   ❌ {flag} not recognized or shows usage")

def test_alternative_execution():
    """Test alternative execution methods"""
    print(f"\n🔬 TESTING ALTERNATIVE EXECUTION METHODS")
    print("=" * 50)

    ip = "192.168.122.10"

    # Create simple test script
    simple_script = '''output("Testing batch execution")
test.batch = true
'''

    with open("/tmp/batch_test.mbl", "w") as f:
        f.write(simple_script)

    # Deploy script
    scp_result = subprocess.run([
        "scp", "-o", "StrictHostKeyChecking=no",
        "/tmp/batch_test.mbl", f"amorphdb@{ip}:/tmp/"
    ], capture_output=True)

    if scp_result.returncode != 0:
        print("❌ Failed to deploy test script")
        return

    # Test different execution methods
    methods = [
        ("Standard input with exit", f"cd /home/amorphdb && (cat /tmp/batch_test.mbl; echo 'exit') | ./amorph -node localhost:5000"),
        ("Heredoc with exit", f"cd /home/amorphdb && ./amorph -node localhost:5000 << 'EOF'\n{simple_script}\nexit\nEOF"),
        ("Script parameter", f"cd /home/amorphdb && ./amorph -node localhost:5000 /tmp/batch_test.mbl"),
        ("Run parameter", f"cd /home/amorphdb && ./amorph -node localhost:5000 -run /tmp/batch_test.mbl")
    ]

    for method_name, command in methods:
        print(f"\n🎯 Testing: {method_name}")

        # Use timeout to prevent hanging
        test_result = subprocess.run([
            "ssh", "-o", "ConnectTimeout=5", f"amorphdb@{ip}", f"timeout 10 {command}"
        ], capture_output=True, text=True, timeout=15)

        print(f"   Return code: {test_result.returncode}")

        if test_result.stdout and "amorph>" not in test_result.stdout:
            print(f"   ✅ Clean output: {test_result.stdout.strip()[:100]}...")
            print(f"   🎉 {method_name} might work!")
        elif test_result.stdout:
            print(f"   ⚠️ Has prompts: {method_name} still interactive")

        if test_result.stderr:
            print(f"   Error: {test_result.stderr.strip()[:100]}")

def main():
    """Run all tests to find batch execution method"""
    print("🔍 FINDING CORRECT BATCH EXECUTION METHOD")
    print("=" * 60)

    check_amorph_help()
    test_batch_flags()
    test_alternative_execution()

    print(f"\n" + "=" * 60)
    print("💡 CONCLUSION:")
    print("Look for methods that produce clean output without 'amorph>' prompts")

if __name__ == "__main__":
    main()