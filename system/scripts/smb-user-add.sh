#!/bin/bash
# Safe SMB user creation wrapper
set -euo pipefail

USERNAME="$1"
PASSWORD="$2"

# Validation - alphanumeric, dash, underscore only, 3-32 chars
if [[ ! "$USERNAME" =~ ^[a-z][a-z0-9_-]{2,31}$ ]]; then
    echo "Error: Username must be 3-32 chars, lowercase alphanumeric + dash/underscore" >&2
    exit 1
fi

if [ ${#PASSWORD} -lt 8 ]; then
    echo "Error: Password must be at least 8 characters" >&2
    exit 1
fi

# Create system user for SMB only - no login, no home
/usr/sbin/useradd \
    --no-create-home \
    --shell /usr/sbin/nologin \
    --comment "SMB Share User" \
    "$USERNAME" || {
        echo "Error: Failed to create user" >&2
        exit 1
    }

# Set SMB password
echo -e "$PASSWORD\n$PASSWORD" | /usr/bin/smbpasswd -a -s "$USERNAME" || {
    # Cleanup on failure
    /usr/sbin/userdel "$USERNAME" 2>/dev/null
    echo "Error: Failed to set SMB password" >&2
    exit 1
}

echo "SMB user '$USERNAME' created successfully"
