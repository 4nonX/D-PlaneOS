#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════
#  D-PlaneOS NixOS Setup Helper
# ═══════════════════════════════════════════════════════════════
#
#  Dieses Script bereitet configuration.nix automatisch vor.
#  Es erledigt die technischen Schritte, die sonst manuell wären.
#
#  Ausführen mit:
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

# ─── Prüfe ob configuration.nix existiert ──────────────────

if [ ! -f "$CONFIG" ]; then
    echo -e "${RED}FEHLER: $CONFIG nicht gefunden.${NC}"
    echo ""
    echo "Hast du die configuration.nix schon kopiert?"
    echo "  sudo cp configuration.nix /etc/nixos/configuration.nix"
    exit 1
fi

echo -e "${GREEN}✓${NC} configuration.nix gefunden"

# ─── Schritt 1: Host-ID generieren ─────────────────────────

echo ""
echo -e "${YELLOW}Schritt 1/4: Host-ID für ZFS${NC}"

CURRENT_HOSTID=$(grep 'networking.hostId' "$CONFIG" | grep -oP '"[^"]*"' | tr -d '"')

if [ "$CURRENT_HOSTID" = "HIER_AENDERN" ]; then
    NEW_HOSTID=$(head -c4 /dev/urandom | od -A none -t x4 | tr -d ' ')
    sed -i "s/\"HIER_AENDERN\"/\"${NEW_HOSTID}\"/" "$CONFIG"
    echo -e "${GREEN}✓${NC} Host-ID generiert und eingetragen: ${CYAN}${NEW_HOSTID}${NC}"
else
    echo -e "${GREEN}✓${NC} Host-ID bereits gesetzt: ${CYAN}${CURRENT_HOSTID}${NC}"
fi

# ─── Schritt 2: Zeitzone prüfen ────────────────────────────

echo ""
echo -e "${YELLOW}Schritt 2/4: Zeitzone${NC}"

CURRENT_TZ=$(grep 'time.timeZone' "$CONFIG" | grep -oP '"[^"]*"' | tr -d '"')
echo "  Aktuelle Einstellung: $CURRENT_TZ"

read -p "  Passt das? [J/n] " tz_ok
if [[ "$tz_ok" =~ ^[Nn] ]]; then
    echo ""
    echo "  Häufige Zeitzonen:"
    echo "    Europe/Berlin     (Deutschland)"
    echo "    Europe/Vienna     (Österreich)"
    echo "    Europe/Zurich     (Schweiz)"
    echo "    America/New_York  (US East)"
    echo "    America/Los_Angeles (US West)"
    echo ""
    read -p "  Neue Zeitzone: " NEW_TZ
    if [ -n "$NEW_TZ" ]; then
        sed -i "s|time.timeZone = \".*\"|time.timeZone = \"${NEW_TZ}\"|" "$CONFIG"
        echo -e "${GREEN}✓${NC} Zeitzone geändert: ${CYAN}${NEW_TZ}${NC}"
    fi
else
    echo -e "${GREEN}✓${NC} Zeitzone beibehalten: ${CYAN}${CURRENT_TZ}${NC}"
fi

# ─── Schritt 3: ZFS Pool-Name ──────────────────────────────

echo ""
echo -e "${YELLOW}Schritt 3/4: ZFS Pool${NC}"

CURRENT_POOL=$(grep 'zpools = ' "$CONFIG" | grep -oP '"[^"]*"' | tr -d '"')
echo "  Aktueller Pool-Name: $CURRENT_POOL"

if command -v zpool &>/dev/null; then
    EXISTING=$(zpool list -H -o name 2>/dev/null || true)
    if [ -n "$EXISTING" ]; then
        echo -e "  ${GREEN}Gefundene ZFS-Pools:${NC} $EXISTING"
    fi
fi

read -p "  Pool-Name ändern? [j/N] " pool_ok
if [[ "$pool_ok" =~ ^[Jj] ]]; then
    read -p "  Neuer Pool-Name: " NEW_POOL
    if [ -n "$NEW_POOL" ]; then
        sed -i "s|zpools = \[ \".*\" \]|zpools = [ \"${NEW_POOL}\" ]|" "$CONFIG"
        echo -e "${GREEN}✓${NC} Pool-Name geändert: ${CYAN}${NEW_POOL}${NC}"
    fi
else
    echo -e "${GREEN}✓${NC} Pool-Name beibehalten: ${CYAN}${CURRENT_POOL}${NC}"
fi

# ─── Schritt 4: Bootloader prüfen ──────────────────────────

echo ""
echo -e "${YELLOW}Schritt 4/4: Bootloader${NC}"

if [ -d /sys/firmware/efi ]; then
    echo -e "  ${GREEN}UEFI erkannt${NC} — Konfiguration passt (systemd-boot)"
else
    echo -e "  ${YELLOW}Kein UEFI erkannt${NC} — wahrscheinlich BIOS/MBR"
    echo ""
    read -p "  Soll ich auf BIOS/MBR umstellen? [j/N] " bios_ok
    if [[ "$bios_ok" =~ ^[Jj] ]]; then
        sed -i 's|^  boot.loader.systemd-boot.enable = true;|  # boot.loader.systemd-boot.enable = true;|' "$CONFIG"
        sed -i 's|^  boot.loader.efi.canTouchEfiVariables = true;|  # boot.loader.efi.canTouchEfiVariables = true;|' "$CONFIG"
        sed -i 's|^  # boot.loader.grub.enable = true;|  boot.loader.grub.enable = true;|' "$CONFIG"
        sed -i 's|^  # boot.loader.grub.device = "/dev/sda";|  boot.loader.grub.device = "/dev/sda";|' "$CONFIG"

        echo ""
        echo "  Verfügbare Disks:"
        lsblk -d -o NAME,SIZE,MODEL | grep -v loop
        echo ""
        read -p "  Boot-Disk (z.B. /dev/sda): " BOOT_DISK
        if [ -n "$BOOT_DISK" ]; then
            sed -i "s|boot.loader.grub.device = \"/dev/sda\"|boot.loader.grub.device = \"${BOOT_DISK}\"|" "$CONFIG"
        fi
        echo -e "${GREEN}✓${NC} Auf BIOS/MBR umgestellt"
    else
        echo -e "${GREEN}✓${NC} Bootloader beibehalten (UEFI)"
    fi
fi

# ─── Zusammenfassung ────────────────────────────────────────

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  Konfiguration vorbereitet!${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""
echo "  Nächster Schritt:"
echo ""
echo -e "    ${CYAN}sudo nixos-rebuild switch${NC}"
echo ""
echo "  Beim ersten Mal wird der Build wahrscheinlich fehlschlagen"
echo "  wegen fehlender Paket-Hashes. Das ist normal!"
echo ""
echo "  So löst du das:"
echo ""
echo "  1. Der Fehler zeigt dir den korrekten Hash:"
echo "       hash mismatch in fixed-output derivation:"
echo "         got: sha256-AbCdEf1234...="
echo ""
echo "  2. Kopiere den 'got:' Hash und ersetze damit den"
echo "     sha256-AAA...-Platzhalter in der configuration.nix:"
echo ""
echo -e "       ${CYAN}sudo nano /etc/nixos/configuration.nix${NC}"
echo "       (Ctrl+W → 'AAAA' suchen → Hash ersetzen)"
echo ""
echo "  3. Nochmal: sudo nixos-rebuild switch"
echo ""
echo "  Nach max. 3 Versuchen läuft alles."
echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
