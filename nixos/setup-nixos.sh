#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════
#  D-PlaneOS NixOS Setup Helper
# ═══════════════════════════════════════════════════════════════
#
#  Prepares your configuration.nix automatically.
#  Handles the technical steps so you don't have to.
#
#  Usage:
#    sudo bash setup-nixos.sh
#
# ═══════════════════════════════════════════════════════════════

set -e

CONFIG="/etc/nixos/configuration.nix"
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}  D-PlaneOS NixOS Setup Helper${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""

# ─── Check if configuration.nix exists ─────────────────────

if [ ! -f "$CONFIG" ]; then
    echo -e "${RED}ERROR: $CONFIG not found.${NC}"
    echo ""
    echo "Did you copy the configuration.nix yet?"
    echo "  sudo cp configuration.nix /etc/nixos/configuration.nix"
    exit 1
fi

echo -e "${GREEN}✓${NC} configuration.nix found"

# ─── Step 1: Generate host ID ──────────────────────────────

echo ""
echo -e "${YELLOW}Step 1/4: ZFS Host ID${NC}"

CURRENT_HOSTID=$(grep 'networking.hostId' "$CONFIG" | grep -oP '"[^"]*"' | tr -d '"')

if [ "$CURRENT_HOSTID" = "CHANGE_ME" ]; then
    NEW_HOSTID=$(head -c4 /dev/urandom | od -A none -t x4 | tr -d ' ')
    sed -i "s/\"CHANGE_ME\"/\"${NEW_HOSTID}\"/" "$CONFIG"
    echo -e "${GREEN}✓${NC} Host ID generated and applied: ${CYAN}${NEW_HOSTID}${NC}"
else
    echo -e "${GREEN}✓${NC} Host ID already set: ${CYAN}${CURRENT_HOSTID}${NC}"
fi

# ─── Step 2: Verify timezone ───────────────────────────────

echo ""
echo -e "${YELLOW}Step 2/4: Timezone${NC}"

CURRENT_TZ=$(grep 'time.timeZone' "$CONFIG" | grep -oP '"[^"]*"' | tr -d '"')
echo "  Current setting: $CURRENT_TZ"

read -p "  Is this correct? [Y/n] " tz_ok
if [[ "$tz_ok" =~ ^[Nn] ]]; then
    echo ""
    echo "  Common timezones:"
    echo "    Europe/Berlin      (Germany)"
    echo "    Europe/London      (UK)"
    echo "    America/New_York   (US East)"
    echo "    America/Los_Angeles (US West)"
    echo "    Asia/Tokyo         (Japan)"
    echo ""
    read -p "  New timezone: " NEW_TZ
    if [ -n "$NEW_TZ" ]; then
        sed -i "s|time.timeZone = \".*\"|time.timeZone = \"${NEW_TZ}\"|" "$CONFIG"
        echo -e "${GREEN}✓${NC} Timezone changed: ${CYAN}${NEW_TZ}${NC}"
    fi
else
    echo -e "${GREEN}✓${NC} Timezone kept: ${CYAN}${CURRENT_TZ}${NC}"
fi

# ─── Step 3: ZFS pool name ─────────────────────────────────

echo ""
echo -e "${YELLOW}Step 3/4: ZFS Pool${NC}"

CURRENT_POOL=$(grep 'zpools = ' "$CONFIG" | grep -oP '"[^"]*"' | tr -d '"')
echo "  Current pool name: $CURRENT_POOL"

if command -v zpool &>/dev/null; then
    EXISTING=$(zpool list -H -o name 2>/dev/null || true)
    if [ -n "$EXISTING" ]; then
        echo -e "  ${GREEN}Detected ZFS pools:${NC} $EXISTING"
    fi
fi

read -p "  Change pool name? [y/N] " pool_ok
if [[ "$pool_ok" =~ ^[Yy] ]]; then
    read -p "  New pool name: " NEW_POOL
    if [ -n "$NEW_POOL" ]; then
        sed -i "s|zpools = \[ \".*\" \]|zpools = [ \"${NEW_POOL}\" ]|" "$CONFIG"
        echo -e "${GREEN}✓${NC} Pool name changed: ${CYAN}${NEW_POOL}${NC}"
    fi
else
    echo -e "${GREEN}✓${NC} Pool name kept: ${CYAN}${CURRENT_POOL}${NC}"
fi

# ─── Step 4: Detect boot loader ────────────────────────────

echo ""
echo -e "${YELLOW}Step 4/4: Boot Loader${NC}"

if [ -d /sys/firmware/efi ]; then
    echo -e "  ${GREEN}UEFI detected${NC} — configuration matches (systemd-boot)"
else
    echo -e "  ${YELLOW}No UEFI detected${NC} — likely BIOS/MBR"
    echo ""
    read -p "  Switch to BIOS/MBR? [y/N] " bios_ok
    if [[ "$bios_ok" =~ ^[Yy] ]]; then
        sed -i 's|^  boot.loader.systemd-boot.enable = true;|  # boot.loader.systemd-boot.enable = true;|' "$CONFIG"
        sed -i 's|^  boot.loader.efi.canTouchEfiVariables = true;|  # boot.loader.efi.canTouchEfiVariables = true;|' "$CONFIG"
        sed -i 's|^  # boot.loader.grub.enable = true;|  boot.loader.grub.enable = true;|' "$CONFIG"
        sed -i 's|^  # boot.loader.grub.device = "/dev/sda";|  boot.loader.grub.device = "/dev/sda";|' "$CONFIG"

        echo ""
        echo "  Available disks:"
        lsblk -d -o NAME,SIZE,MODEL | grep -v loop
        echo ""
        read -p "  Boot disk (e.g. /dev/sda): " BOOT_DISK
        if [ -n "$BOOT_DISK" ]; then
            sed -i "s|boot.loader.grub.device = \"/dev/sda\"|boot.loader.grub.device = \"${BOOT_DISK}\"|" "$CONFIG"
        fi
        echo -e "${GREEN}✓${NC} Switched to BIOS/MBR"
    else
        echo -e "${GREEN}✓${NC} Boot loader kept (UEFI)"
    fi
fi

# ─── Summary ────────────────────────────────────────────────

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  Configuration ready!${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""
echo "  Next step:"
echo ""
echo -e "    ${CYAN}sudo nixos-rebuild switch --flake .#dplaneos${NC}"
echo ""
echo "  The first build will likely fail due to missing package"
echo "  hashes. This is normal!"
echo ""
echo "  How to fix:"
echo ""
echo "  1. The error shows the correct hash:"
echo "       hash mismatch in fixed-output derivation:"
echo "         got: sha256-AbCdEf1234...="
echo ""
echo "  2. Copy the 'got:' hash and replace the placeholder"
echo "     in flake.nix (or configuration.nix for standalone):"
echo ""
echo -e "       ${CYAN}sudo nano flake.nix${NC}"
echo "       (Ctrl+W → search 'null' or 'AAAA' → paste hash)"
echo ""
echo "  3. Rebuild: sudo nixos-rebuild switch --flake .#dplaneos"
echo ""
echo "  After max 3 attempts everything will build."
echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
