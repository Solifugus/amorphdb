#!/usr/bin/env python3
"""
AmorphDB Project Cleanup Script

This script organizes the AmorphDB project according to Go project conventions:
- Moves documentation to docs/
- Moves test files to tests/
- Moves utility scripts to scripts/
- Removes unnecessary/outdated files
"""

import os
import shutil
import glob
from pathlib import Path

def main():
    project_root = Path(__file__).parent.parent
    os.chdir(project_root)

    print("🧹 Cleaning up AmorphDB project structure...")

    # Create directory structure
    dirs_to_create = [
        "docs",
        "tests/phase3",
        "tests/integration",
        "tests/unit",
        "scripts"
    ]

    for dir_path in dirs_to_create:
        Path(dir_path).mkdir(parents=True, exist_ok=True)
        print(f"✅ Created directory: {dir_path}")

    # Move Phase 3 test files (test_*.mbl files)
    phase3_tests = glob.glob("test_*.mbl")
    for test_file in phase3_tests:
        dest = f"tests/phase3/{test_file}"
        if not os.path.exists(dest):
            shutil.move(test_file, dest)
            print(f"📁 Moved {test_file} → {dest}")

    # Move other MBL test files
    other_mbl_files = [f for f in glob.glob("*.mbl") if not f.startswith("test_")]
    for mbl_file in other_mbl_files:
        dest = f"tests/unit/{mbl_file}"
        if not os.path.exists(dest):
            shutil.move(mbl_file, dest)
            print(f"📁 Moved {mbl_file} → {dest}")

    # Move Python scripts
    python_scripts = glob.glob("*.py")
    for script in python_scripts:
        if script == "scripts/cleanup_project.py":  # Don't move ourselves
            continue
        dest = f"scripts/{script}"
        if not os.path.exists(dest):
            shutil.move(script, dest)
            print(f"📁 Moved {script} → {dest}")

    # Move status/documentation files to docs
    doc_files = [
        "HERITABILITY_STATUS.md",
        "REPL_STATUS.md",
        "STEP8_STATUS.md",
        "PHASE3_PRODUCTION_READINESS.md",
        "PHASE3_VM_INTEGRATION.md",
        "VM_TESTING_LOG.md",
        "VM_SUCCESS_LOG.md",
        "VM_PROGRESS_SUMMARY.md",
        "VM_DEPLOYMENT_STATUS.md",
        "PRODUCTION_DEPLOYMENT_METHODOLOGY.md",
        "SCP_DEPLOYMENT_SUCCESS.md",
        "service_integration_summary.md",
        "amorphdb_development_plan.md",
        "amorphdb_design_original.md"
    ]

    for doc_file in doc_files:
        if os.path.exists(doc_file):
            dest = f"docs/{doc_file}"
            if not os.path.exists(dest):
                shutil.move(doc_file, dest)
                print(f"📁 Moved {doc_file} → {dest}")

    # Clean up temporary and unnecessary files
    temp_files = [
        "temp.txt",
        "problematic_inputs.txt",
        "notes.txt",
        "*.go",  # Only if they're at root level and should be elsewhere
        "*.log",
        "*.tmp"
    ]

    for pattern in temp_files:
        for temp_file in glob.glob(pattern):
            # Skip if it's a core Go file that should stay at root
            if temp_file.endswith('.go'):
                # Only remove root-level demo/test Go files
                if any(name in temp_file for name in ['demo', 'test', 'simple']):
                    dest = f"tests/integration/{temp_file}"
                    if not os.path.exists(dest):
                        shutil.move(temp_file, dest)
                        print(f"📁 Moved {temp_file} → {dest}")
            else:
                os.remove(temp_file)
                print(f"🗑️ Removed temporary file: {temp_file}")

    # Move shell scripts to scripts
    shell_scripts = glob.glob("*.sh")
    for script in shell_scripts:
        dest = f"scripts/{script}"
        if not os.path.exists(dest):
            shutil.move(script, dest)
            print(f"📁 Moved {script} → {dest}")

    print("\n✨ Project cleanup complete!")
    print("\n📁 Final structure:")
    print("├── docs/           # Documentation and design documents")
    print("├── tests/          # Test files organized by type")
    print("│   ├── phase3/     # Phase 3 production readiness tests")
    print("│   ├── integration/# Integration tests")
    print("│   └── unit/       # Unit tests and simple examples")
    print("├── scripts/        # Build and deployment scripts")
    print("├── cmd/            # Main applications (amorphd, amorph, amorphctl)")
    print("├── internal/       # Private Go packages")
    print("└── README.md       # Project overview")

    print("\n🎯 Key files preserved:")
    print("- docs/amorphdb_design.md (Core system design)")
    print("- docs/AmorphDB_White_Paper.md (Executive white paper)")
    print("- docs/PHASE3_EXECUTION_STATUS.md (Testing validation)")
    print("- README.md (Project overview)")

if __name__ == "__main__":
    main()