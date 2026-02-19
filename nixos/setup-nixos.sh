#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════
#  D-PlaneOS NixOS Setup Helper
# ═══════════════════════════════════════════════════════════════
#
#  Prepares configuration.nix automatically:
#    - Generates ZFS host ID
#    - Auto-detects ZFS pools
#    - Detects UEFI vs BIOS
#    - Sets timezone
#    - Prefetches Go vendor hash (fixes first-build failure)
#
#  Usage:  sudo bash setup-nixos.sh
#
# ═══════════════════════════════════════════════════════════════

set -e

# ─── Configuration ─────────────────────────────────────────
NIXOS_DIR="$(cd "$(dirname "$0")" && pwd)"
CONFIG="$NIXOS_DIR/configuration.nix"
FLAKE="$NIXOS_DIR/flake.nix"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}  D-PlaneOS NixOS Setup${NC}"
echo -e "${CYAN}  The Immutable NAS: NixOS + ZFS + GitOps${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""

if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}Error: Run with sudo${NC}"
    echo "  sudo bash setup-nixos.sh"
    exit 1
fi

if [ ! -f "$CONFIG" ]; then
    echo -e "${RED}Error: configuration.nix not found at $CONFIG${NC}"
    exit 1
fi

# ═══════════════════════════════════════════════════════════════
#  Step 1: ZFS Host ID (required — ZFS won't import without it)
# ═══════════════════════════════════════════════════════════════

echo -e "${YELLOW}Step 1/5: ZFS Host ID${NC}"

CURRENT_HOSTID=$(grep 'networking.hostId' "$CONFIG" | grep -oP '"[^"]*"' | tr -d '"')

if [ "$CURRENT_HOSTID" = "CHANGE_ME" ]; then
    # Use existing machine hostid if available, otherwise generate
    if [ -f /etc/machine-id ]; then
        NEW_HOSTID=$(head -c 8 /etc/machine-id)
    else
        NEW_HOSTID=$(head -c4 /dev/urandom | od -A none -t x4 | tr -d ' ')
    fi
    sed -i "s/\"CHANGE_ME\"/\"${NEW_HOSTID}\"/" "$CONFIG"
    echo -e "  ${GREEN}✓${NC} Generated host ID: ${CYAN}${NEW_HOSTID}${NC}"
else
    echo -e "  ${GREEN}✓${NC} Already set: ${CYAN}${CURRENT_HOSTID}${NC}"
fi

# ═══════════════════════════════════════════════════════════════
#  Step 2: ZFS Pool Detection
# ═══════════════════════════════════════════════════════════════

echo ""
echo -e "${YELLOW}Step 2/5: ZFS Pools${NC}"

CURRENT_POOLS=$(grep 'zpools = ' "$CONFIG" | grep -oP '\[ "[^"]*"' | tr -d '[ "')

if command -v zpool &>/dev/null; then
    DETECTED_POOLS=$(zpool list -H -o name 2>/dev/null | tr '\n' ' ' | sed 's/ *$//')

    if [ -n "$DETECTED_POOLS" ]; then
        echo "  Detected pools: ${DETECTED_POOLS}"

        # Format as Nix list: [ "pool1" "pool2" ]
        NIX_POOLS="[ $(echo "$DETECTED_POOLS" | sed 's/\([^ ]*\)/"\1"/g') ]"

        sed -i "s|zpools = \[.*\];|zpools = ${NIX_POOLS};|" "$CONFIG"
        echo -e "  ${GREEN}✓${NC} Pool list updated: ${CYAN}${NIX_POOLS}${NC}"
    else
        echo -e "  ${YELLOW}!${NC} No ZFS pools found (create them first with zpool create)"
        echo "  Current setting: ${CURRENT_POOLS:-tank}"
    fi
else
    echo -e "  ${YELLOW}!${NC} ZFS not installed yet (will be after first nixos-rebuild)"
    echo "  Current setting: ${CURRENT_POOLS:-tank}"
    echo "  You can re-run this script after creating pools."
fi

# ═══════════════════════════════════════════════════════════════
#  Step 3: Timezone
# ═══════════════════════════════════════════════════════════════

echo ""
echo -e "${YELLOW}Step 3/5: Timezone${NC}"

CURRENT_TZ=$(grep 'time.timeZone' "$CONFIG" | grep -oP '"[^"]*"' | tr -d '"')

# Try to detect from timedatectl
DETECTED_TZ=""
if command -v timedatectl &>/dev/null; then
    DETECTED_TZ=$(timedatectl show -p Timezone --value 2>/dev/null || true)
fi

if [ -n "$DETECTED_TZ" ] && [ "$DETECTED_TZ" != "$CURRENT_TZ" ]; then
    echo "  Detected: $DETECTED_TZ (config has: $CURRENT_TZ)"
    read -p "  Use detected timezone? [Y/n] " tz_use
    if [[ ! "$tz_use" =~ ^[Nn] ]]; then
        sed -i "s|time.timeZone = \".*\"|time.timeZone = \"${DETECTED_TZ}\"|" "$CONFIG"
        echo -e "  ${GREEN}✓${NC} Timezone: ${CYAN}${DETECTED_TZ}${NC}"
    else
        echo -e "  ${GREEN}✓${NC} Keeping: ${CYAN}${CURRENT_TZ}${NC}"
    fi
else
    echo -e "  ${GREEN}✓${NC} Timezone: ${CYAN}${CURRENT_TZ}${NC}"
    read -p "  Change? [y/N] " tz_change
    if [[ "$tz_change" =~ ^[Yy] ]]; then
        read -p "  New timezone (e.g. America/New_York): " NEW_TZ
        if [ -n "$NEW_TZ" ]; then
            sed -i "s|time.timeZone = \".*\"|time.timeZone = \"${NEW_TZ}\"|" "$CONFIG"
            echo -e "  ${GREEN}✓${NC} Timezone: ${CYAN}${NEW_TZ}${NC}"
        fi
    fi
fi

# ═══════════════════════════════════════════════════════════════
#  Step 4: Boot Loader Detection
# ═══════════════════════════════════════════════════════════════

echo ""
echo -e "${YELLOW}Step 4/5: Boot Loader${NC}"

if [ -d /sys/firmware/efi ]; then
    echo -e "  ${GREEN}✓${NC} UEFI detected — systemd-boot (default)"
else
    echo -e "  ${YELLOW}!${NC} No UEFI detected — switching to GRUB (Legacy BIOS)"

    # Comment out UEFI, uncomment GRUB
    sed -i 's|^  boot.loader.systemd-boot.enable = true;|  # boot.loader.systemd-boot.enable = true;|' "$CONFIG"
    sed -i 's|^  boot.loader.efi.canTouchEfiVariables = true;|  # boot.loader.efi.canTouchEfiVariables = true;|' "$CONFIG"
    sed -i 's|^  # boot.loader.grub.enable = true;|  boot.loader.grub.enable = true;|' "$CONFIG"
    sed -i 's|^  # boot.loader.grub.device = "/dev/sda";|  boot.loader.grub.device = "/dev/sda";|' "$CONFIG"

    echo ""
    echo "  Available disks:"
    lsblk -d -o NAME,SIZE,MODEL 2>/dev/null | grep -v loop || true
    echo ""
    read -p "  Boot disk [/dev/sda]: " BOOT_DISK
    BOOT_DISK=${BOOT_DISK:-/dev/sda}
    sed -i "s|boot.loader.grub.device = \"/dev/sda\"|boot.loader.grub.device = \"${BOOT_DISK}\"|" "$CONFIG"
    echo -e "  ${GREEN}✓${NC} GRUB on ${CYAN}${BOOT_DISK}${NC}"
fi

# ═══════════════════════════════════════════════════════════════
#  Step 5: Vendor Hash (prevents first-build failure)
# ═══════════════════════════════════════════════════════════════

echo ""
echo -e "${YELLOW}Step 5/5: Go Vendor Hash${NC}"

if [ -f "$FLAKE" ]; then
    CURRENT_HASH=$(grep 'vendorHash' "$FLAKE" | grep -oP '"[^"]*"' | tr -d '"' || echo "null")

    if [ "$CURRENT_HASH" = "null" ] || [ -z "$CURRENT_HASH" ]; then
        echo "  vendorHash is null — first build will fail without it."
        echo ""
        echo "  Two options:"
        echo "    a) Try building now — Nix will show the correct hash"
        echo "    b) Skip — you'll need to fix it manually after first build attempt"
        echo ""
        read -p "  Try prefetching? [Y/n] " prefetch_ok

        if [[ ! "$prefetch_ok" =~ ^[Nn] ]]; then
            echo "  Attempting build to capture hash (this may take a few minutes)..."

            # Try a build, capture the hash from the error
            BUILD_OUTPUT=$(nixos-rebuild build --flake "$NIXOS_DIR#dplaneos" 2>&1 || true)
            GOT_HASH=$(echo "$BUILD_OUTPUT" | grep -oP 'got:\s+\Ksha256-[A-Za-z0-9+/=]+' | head -1)

            if [ -n "$GOT_HASH" ]; then
                sed -i "s|vendorHash = null;|vendorHash = \"${GOT_HASH}\";|" "$FLAKE"
                echo -e "  ${GREEN}✓${NC} Hash captured and applied: ${CYAN}${GOT_HASH}${NC}"
                echo "  Next build should succeed."
            else
                echo -e "  ${YELLOW}!${NC} Could not capture hash automatically."
                echo "  After first build attempt, look for:"
                echo "    got: sha256-XXXXX..."
                echo "  Then: sed -i 's|vendorHash = null|vendorHash = \"sha256-XXXXX...\"|' flake.nix"
            fi
        else
            echo -e "  ${YELLOW}!${NC} Skipped. First build will fail — see instructions below."
        fi
    else
        echo -e "  ${GREEN}✓${NC} Already set: ${CYAN}${CURRENT_HASH}${NC}"
    fi
else
    echo -e "  ${YELLOW}!${NC} flake.nix not found — skipping"
fi

# ═══════════════════════════════════════════════════════════════
#  Done
# ═══════════════════════════════════════════════════════════════

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  Setup complete!${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""
echo "  Build your immutable NAS:"
echo ""
echo -e "    ${BOLD}sudo nixos-rebuild switch --flake .#dplaneos${NC}"
echo ""

VENDOR_HASH=$(grep 'vendorHash' "$FLAKE" 2>/dev/null | grep -oP '"[^"]*"' | tr -d '"' || echo "null")
if [ "$VENDOR_HASH" = "null" ] || [ -z "$VENDOR_HASH" ]; then
    echo -e "  ${YELLOW}Note:${NC} First build will fail (missing vendor hash)."
    echo "  This is normal for Nix flakes with Go modules."
    echo ""
    echo "  1. Run the build command above"
    echo "  2. Copy the sha256-... hash from the error"
    echo "  3. Edit flake.nix: replace 'null' with the hash"
    echo "  4. Build again — it will succeed"
    echo ""
fi

echo "  After install:"
echo "    Web UI → http://$(hostname -I 2>/dev/null | awk '{print $1}' || echo 'your-ip')"
echo "    Recovery → sudo dplaneos-recovery"
echo "    Rollback → sudo nixos-rebuild switch --rollback"
echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
