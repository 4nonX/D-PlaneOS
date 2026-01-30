#!/bin/bash
# Safe SMB user deletion wrapper
set -euo pipefail

USERNAME="$1"

# Validation
if [[ ! "$USERNAME" =~ ^[a-z][a-z0-9_-]{2,31}$ ]]; then
    echo "Error: Invalid username format" >&2
    exit 1
fi

# Remove from Samba
/usr/bin/smbpasswd -x "$USERNAME" 2>/dev/null || true

# Remove system user
/usr/sbin/userdel "$USERNAME" || {
    echo "Error: Failed to delete user" >&2
    exit 1
}

echo "SMB user '$USERNAME' deleted successfully"
