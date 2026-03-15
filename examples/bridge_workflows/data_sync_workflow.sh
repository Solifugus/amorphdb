#!/bin/bash
# Data Synchronization Workflow
#
# This script demonstrates how to synchronize data between meshes
# using bridge connections. Useful for keeping configuration
# or shared state synchronized across multiple meshes.

set -euo pipefail

# Configuration
SOCKET_PATH="${AMORPHDB_SOCKET:-/var/run/amorphdb.sock}"
PARTNER_MESH="${PARTNER_MESH:-}"
SYNC_CONFIG="${SYNC_CONFIG:-/etc/amorphdb/sync.conf}"

# Helper function
adb() {
    amorphctl -s "$SOCKET_PATH" "$@"
}

echo "🔄 AmorphDB Data Synchronization Workflow"
echo "=========================================="

# Load sync configuration
if [ -f "$SYNC_CONFIG" ]; then
    source "$SYNC_CONFIG"
    echo "📋 Loaded sync configuration from $SYNC_CONFIG"
else
    echo "⚠️  No sync configuration found at $SYNC_CONFIG"
    echo "   Using default configuration..."

    # Default sync paths
    SYNC_PATHS=(
        "world.shared.config"
        "world.shared.templates"
        "world.shared.policies"
    )

    # Default partner mesh (from command line or discovery)
    if [ -z "$PARTNER_MESH" ]; then
        # Try to auto-discover partner mesh
        BRIDGES=$(adb bridge list 2>/dev/null || echo "")
        if [ -n "$BRIDGES" ]; then
            PARTNER_MESH=$(echo "$BRIDGES" | head -1 | awk '{print $1}')
            echo "🔍 Auto-discovered partner mesh: $PARTNER_MESH"
        else
            echo "❌ No partner mesh specified and no bridges found"
            echo "Usage: PARTNER_MESH=mesh-name $0"
            echo "   or: establish a bridge first with 'amorphctl bridge <address>'"
            exit 1
        fi
    fi
fi

echo "📊 Sync Configuration:"
echo "  Partner mesh: $PARTNER_MESH"
echo "  Sync paths: ${SYNC_PATHS[*]}"

# Verify bridge connection
echo ""
echo "🔗 Verifying bridge connection..."
if ! adb bridge status "$PARTNER_MESH" >/dev/null 2>&1; then
    echo "❌ No bridge to $PARTNER_MESH"
    echo "   Establish bridge first: amorphctl bridge <partner-address>"
    exit 1
fi

echo "✅ Bridge to $PARTNER_MESH is active"

# Create sync state tracking
SYNC_STATE_DIR="/var/lib/amorphdb/sync"
mkdir -p "$SYNC_STATE_DIR"

# Sync function
sync_path() {
    local path="$1"
    local direction="${2:-bidirectional}"  # bidirectional, to_partner, from_partner

    echo "🔄 Syncing path: $path ($direction)"

    local local_path="$path"
    local partner_path="my.$PARTNER_MESH.$path"
    local state_file="$SYNC_STATE_DIR/${path//\//_}.state"

    # Get current timestamps
    local local_timestamp=""
    local partner_timestamp=""

    if local_data=$(adb get "$local_path" 2>/dev/null); then
        local_timestamp=$(adb get "$local_path.timestamp" 2>/dev/null || echo "0")
    fi

    if partner_data=$(adb get "$partner_path" 2>/dev/null); then
        partner_timestamp=$(adb get "$partner_path.timestamp" 2>/dev/null || echo "0")
    fi

    # Load previous sync state
    local last_sync_time="0"
    if [ -f "$state_file" ]; then
        last_sync_time=$(cat "$state_file")
    fi

    # Determine sync action
    local action="none"

    if [ -n "$local_data" ] && [ -z "$partner_data" ]; then
        if [ "$direction" != "from_partner" ]; then
            action="copy_to_partner"
        fi
    elif [ -z "$local_data" ] && [ -n "$partner_data" ]; then
        if [ "$direction" != "to_partner" ]; then
            action="copy_from_partner"
        fi
    elif [ -n "$local_data" ] && [ -n "$partner_data" ]; then
        # Both exist - compare timestamps
        if [ "$local_timestamp" -gt "$partner_timestamp" ] && [ "$local_timestamp" -gt "$last_sync_time" ]; then
            if [ "$direction" != "from_partner" ]; then
                action="copy_to_partner"
            fi
        elif [ "$partner_timestamp" -gt "$local_timestamp" ] && [ "$partner_timestamp" -gt "$last_sync_time" ]; then
            if [ "$direction" != "to_partner" ]; then
                action="copy_from_partner"
            fi
        fi
    fi

    # Execute sync action
    case "$action" in
        copy_to_partner)
            echo "  → Copying to partner mesh"
            if adb set "$partner_path" "$local_data"; then
                echo "    ✅ Copy successful"
                echo "$(date +%s)" > "$state_file"
            else
                echo "    ❌ Copy failed"
                return 1
            fi
            ;;
        copy_from_partner)
            echo "  ← Copying from partner mesh"
            if adb set "$local_path" "$partner_data"; then
                echo "    ✅ Copy successful"
                echo "$(date +%s)" > "$state_file"
            else
                echo "    ❌ Copy failed"
                return 1
            fi
            ;;
        none)
            echo "  ✅ Already synchronized"
            ;;
    esac
}

# Perform synchronization
echo ""
echo "🔄 Starting synchronization..."

SYNC_SUCCESS=0
SYNC_TOTAL=${#SYNC_PATHS[@]}

for path in "${SYNC_PATHS[@]}"; do
    if sync_path "$path"; then
        ((SYNC_SUCCESS++))
    fi
done

echo ""
echo "📊 Synchronization Summary:"
echo "  Total paths: $SYNC_TOTAL"
echo "  Successful: $SYNC_SUCCESS"
echo "  Failed: $((SYNC_TOTAL - SYNC_SUCCESS))"

# Create sync report
SYNC_REPORT="/tmp/amorphdb_sync_$(date +%Y%m%d_%H%M%S).log"

cat > "$SYNC_REPORT" << EOF
AmorphDB Sync Report
===================
Date: $(date)
Partner Mesh: $PARTNER_MESH
Paths Synced: ${SYNC_PATHS[*]}
Success Rate: $SYNC_SUCCESS/$SYNC_TOTAL

Sync Details:
EOF

for path in "${SYNC_PATHS[@]}"; do
    echo "  $path: $([ -f "$SYNC_STATE_DIR/${path//\//_}.state" ] && echo "synced" || echo "pending")" >> "$SYNC_REPORT"
done

echo "📄 Sync report saved to: $SYNC_REPORT"

# Set up automated sync (if requested)
if [ "${AUTO_SYNC:-false}" = "true" ]; then
    echo ""
    echo "⚙️  Setting up automated sync..."

    SYNC_SCRIPT="/usr/local/bin/amorphdb_auto_sync_$PARTNER_MESH.sh"

    cat > "$SYNC_SCRIPT" << EOF
#!/bin/bash
# Auto-generated sync script for $PARTNER_MESH
export PARTNER_MESH="$PARTNER_MESH"
export SYNC_PATHS=(${SYNC_PATHS[*]})
exec "$0"
EOF

    chmod +x "$SYNC_SCRIPT"

    # Add to crontab (every 5 minutes)
    (crontab -l 2>/dev/null || true; echo "*/5 * * * * $SYNC_SCRIPT") | crontab -

    echo "✅ Automated sync configured (runs every 5 minutes)"
    echo "   Script: $SYNC_SCRIPT"
    echo "   To disable: crontab -e and remove the line"
fi

# Verification
echo ""
echo "🔍 Verification - checking sync consistency..."

ALL_CONSISTENT=true
for path in "${SYNC_PATHS[@]}"; do
    local_data=$(adb get "$path" 2>/dev/null || echo "")
    partner_data=$(adb get "my.$PARTNER_MESH.$path" 2>/dev/null || echo "")

    if [ "$local_data" = "$partner_data" ]; then
        echo "  ✅ $path: consistent"
    else
        echo "  ❌ $path: inconsistent"
        ALL_CONSISTENT=false
    fi
done

if [ "$ALL_CONSISTENT" = "true" ]; then
    echo ""
    echo "🎉 All configured paths are synchronized!"
else
    echo ""
    echo "⚠️  Some paths remain inconsistent. Check sync permissions and connectivity."
fi

echo ""
echo "Useful commands for ongoing sync management:"
echo "  adb bridge status $PARTNER_MESH    # Check bridge health"
echo "  cat $SYNC_REPORT                   # Review sync report"
echo "  tail -f /var/log/amorphdb.log      # Monitor sync activity"