#!/usr/bin/env bash
#
# DPlaneOS Recovery CLI
# 
# Emergency system recovery tool for when web UI is unavailable.
# Can be run from SSH or console (TTY).
#
# Usage: sudo dplaneos-recovery
#

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

# Check root
if [ "$EUID" -ne 0 ]; then
    echo "This tool must be run as root"
    echo "Usage: sudo dplaneos-recovery"
    exit 1
fi

# Check if dialog is available
if ! command -v dialog &>/dev/null; then
    echo "dialog is not available - install it via: nix-env -iA nixpkgs.dialog"
fi

DIALOG_OK=0
DIALOG_CANCEL=1
DIALOG_ESC=255

show_main_menu() {
    while true; do
        choice=$(dialog --clear --title "DPlaneOS Recovery CLI" \
            --menu "Select an option:" 22 60 15 \
            1 "System Status" \
            2 "Check Services" \
            3 "Restart Services" \
            4 "Check Database" \
            5 "Reset Admin Password" \
            6 "Check ZFS Pools" \
            7 "Import/Export Pool" \
            8 "Check Network" \
            9 "Fix Permissions" \
            10 "UPS Status" \
            11 "View Logs" \
            12 "Run Diagnostics" \
            13 "Emergency Shutdown" \
            14 "Exit" \
            3>&1 1>&2 2>&3)
        
        result=$?
        
        if [ $result -ne $DIALOG_OK ]; then
            clear
            exit 0
        fi
        
        case $choice in
            1) show_system_status ;;
            2) check_services ;;
            3) restart_services ;;
            4) check_database ;;
            5) reset_admin_password ;;
            6) check_zfs_pools ;;
            7) import_export_pool ;;
            8) check_network ;;
            9) fix_permissions ;;
            10) check_ups_status ;;
            11) view_logs ;;
            12) run_diagnostics ;;
            13) emergency_shutdown ;;
            14) clear; exit 0 ;;
        esac
    done
}

show_system_status() {
    STATUS_TEXT=$(cat <<EOF
System Information:
-------------------
Hostname: $(hostname)
OS: $(cat /etc/os-release | grep PRETTY_NAME | cut -d= -f2 | tr -d '"')
Kernel: $(uname -r)
Uptime: $(uptime -p)

Resources:
----------
CPU Cores: $(nproc)
RAM Total: $(free -h | awk '/^Mem:/{print $2}')
RAM Used: $(free -h | awk '/^Mem:/{print $3}')
Disk Used: $(df -h / | awk 'NR==2{print $5}')

DPlaneOS:
----------
Installation: $([ -d /opt/dplaneos ] && echo "Found" || echo "Not found")
Database: $([ -f /var/lib/dplaneos/dplaneos.db ] && echo "Found" || echo "Not found")
nginx: $(systemctl is-active nginx 2>/dev/null || echo "not running")
Go Daemon: $(systemctl is-active dplaned 2>/dev/null || echo "not running")
EOF
)
    
    dialog --title "System Status" --msgbox "$STATUS_TEXT" 25 70
}

check_services() {
    SERVICES_TEXT=$(cat <<EOF
Service Status:
===============

nginx: $(systemctl is-active nginx 2>/dev/null || echo "STOPPED")
  $(systemctl status nginx 2>&1 | head -3 | tail -2)

Go Daemon: $(systemctl is-active dplaned 2>/dev/null || echo "STOPPED")
  $(systemctl status dplaned 2>&1 | head -3 | tail -2)

Ports:
------
Port 80: $(netstat -tuln | grep -q ":80 " && echo "OPEN" || echo "CLOSED")
Port 443: $(netstat -tuln | grep -q ":443 " && echo "OPEN" || echo "CLOSED")
EOF
)
    
    dialog --title "Service Status" --msgbox "$SERVICES_TEXT" 20 70
}

restart_services() {
    dialog --infobox "Restarting services..." 5 40

    local out=""
    systemctl restart nginx 2>&1  && out+="nginx: restarted OK\n"  || out+="nginx: restart FAILED\n"
    systemctl restart dplaned 2>&1 && out+="dplaned: restarted OK\n" || out+="dplaned: restart FAILED\n"
    sleep 1
    dialog --title "Restart Services" --msgbox "$(printf "%b" "$out")" 10 50
}

check_database() {
    local dsn="postgres://dplaneos@localhost/dplaneos?sslmode=disable"
    local pg_status conn_test user_count session_count

    systemctl is-active --quiet postgresql && pg_status="RUNNING" || pg_status="STOPPED"

    psql "$dsn" -c "SELECT 1" >/dev/null 2>&1 \
        && conn_test="Connection: OK" \
        || conn_test="Connection: FAILED"

    user_count=$(psql "$dsn" -t -c "SELECT COUNT(*) FROM users" 2>/dev/null \
        | tr -d '[:space:]' || echo "N/A")
    session_count=$(psql "$dsn" -t -c \
        "SELECT COUNT(*) FROM sessions WHERE expires_at > EXTRACT(EPOCH FROM NOW())" \
        2>/dev/null | tr -d '[:space:]' || echo "N/A")

    dialog --title "Database Status" --msgbox \
"Database Status:
================

PostgreSQL service: $pg_status
$conn_test
Active users:       $user_count
Active sessions:    $session_count" 14 55
}

reset_admin_password() {
    dialog --title "Reset Admin Password" \
        --yesno "Reset the 'admin' account password in PostgreSQL?\n\nProceed?" \
        8 55 || return

    local new_pass
    new_pass=$(dialog --title "Reset Admin Password" \
        --passwordbox "New admin password (min 8 chars):" \
        8 55 3>&1 1>&2 2>&3) || return

    if [ "${#new_pass}" -lt 8 ]; then
        dialog --title "Error" --msgbox "Password must be at least 8 characters." 6 45
        return
    fi

    local confirm_pass
    confirm_pass=$(dialog --title "Reset Admin Password" \
        --passwordbox "Confirm new password:" \
        8 55 3>&1 1>&2 2>&3) || return

    if [ "$new_pass" != "$confirm_pass" ]; then
        dialog --title "Error" --msgbox "Passwords do not match." 6 45
        return
    fi

    local pass_tmp hash
    pass_tmp=$(mktemp)
    chmod 600 "$pass_tmp"
    printf '%s' "$new_pass" > "$pass_tmp"

    hash=$(python3 -c "
import bcrypt, sys
with open(sys.argv[1], 'rb') as f:
    pw = f.read().strip()
print(bcrypt.hashpw(pw, bcrypt.gensalt(rounds=12)).decode())
" "$pass_tmp" 2>/dev/null)
    rm -f "$pass_tmp"

    if [ -z "$hash" ]; then
        dialog --title "Error" --msgbox "Failed to hash password.\n(Is python3 with bcrypt installed?)" 7 55
        return
    fi

    local dsn="postgres://dplaneos@localhost/dplaneos?sslmode=disable"
    if psql "$dsn" -c \
        "UPDATE users SET password_hash='$hash', must_change_password=0 WHERE username='admin'" \
        >/dev/null 2>&1; then
        dialog --title "Success" \
            --msgbox "Admin password reset successfully.\n\nYou can now log in at the web UI." 8 55
    else
        dialog --title "Error" \
            --msgbox "Failed to update password in database.\nCheck that PostgreSQL is running." 8 55
    fi
}

check_zfs_pools() {
    local out
    out=$(zpool status 2>&1 || echo "zpool failed - ZFS module may not be loaded")
    dialog --title "ZFS Pool Status" --msgbox "$out" 30 80
}

import_export_pool() {
    local action
    action=$(dialog --title "ZFS Import/Export" \
        --menu "Select action:" 10 50 3 \
        1 "Import a pool" \
        2 "Export a pool" \
        3 "Back" \
        3>&1 1>&2 2>&3) || return

    case "$action" in
        1)
            local available
            available=$(zpool import 2>&1)
            dialog --title "Importable Pools" --msgbox "$available" 20 70

            local pool_name
            pool_name=$(dialog --title "Import Pool" \
                --inputbox "Pool name to import:" \
                8 50 3>&1 1>&2 2>&3) || return

            [ -z "$pool_name" ] && return
            local result
            if result=$(zpool import "$pool_name" 2>&1); then
                dialog --title "Import" --msgbox "Pool '$pool_name' imported successfully." 7 55
            else
                dialog --title "Import Failed" --msgbox "Failed:\n$result" 10 60
            fi
            ;;
        2)
            local pool_name
            pool_name=$(dialog --title "Export Pool" \
                --inputbox "Pool name to export:" \
                8 50 3>&1 1>&2 2>&3) || return

            [ -z "$pool_name" ] && return
            dialog --title "Confirm Export" \
                --yesno "Export '$pool_name'? All datasets will be unmounted." \
                7 60 || return

            local result
            if result=$(zpool export "$pool_name" 2>&1); then
                dialog --title "Export" --msgbox "Pool '$pool_name' exported successfully." 7 55
            else
                dialog --title "Export Failed" --msgbox "Failed:\n$result" 10 60
            fi
            ;;
    esac
}

check_network() {
    local out
    out="Network Interfaces:
===================
$(ip addr 2>&1)

Default Routes:
===============
$(ip route 2>&1)"
    dialog --title "Network Status" --msgbox "$out" 30 80
}

fix_permissions() {
    dialog --title "Fix Permissions" \
        --yesno "Reset ownership on DPlaneOS directories?\n\n  /var/lib/dplaneos\n  /var/log/dplaneos\n  /etc/dplaneos" \
        10 60 || return

    local out=""
    mkdir -p /var/lib/dplaneos /var/log/dplaneos /etc/dplaneos

    chown -R dplaneos:dplaneos /var/lib/dplaneos 2>/dev/null \
        && out+="OK  /var/lib/dplaneos\n" \
        || out+="WARN /var/lib/dplaneos (user 'dplaneos' may not exist)\n"
    chmod 750 /var/lib/dplaneos 2>/dev/null

    chown -R dplaneos:dplaneos /var/log/dplaneos 2>/dev/null \
        && out+="OK  /var/log/dplaneos\n" \
        || out+="WARN /var/log/dplaneos\n"
    chmod 755 /var/log/dplaneos 2>/dev/null

    chown -R root:root /etc/dplaneos 2>/dev/null \
        && out+="OK  /etc/dplaneos\n" \
        || out+="WARN /etc/dplaneos\n"
    chmod 750 /etc/dplaneos 2>/dev/null

    dialog --title "Fix Permissions" --msgbox "$(printf "%b" "$out")" 12 60
}

check_ups_status() {
    local out
    if command -v upsc &>/dev/null; then
        local ups_list
        ups_list=$(upsc -l 2>/dev/null | head -1)
        if [ -n "$ups_list" ]; then
            out=$(upsc "$ups_list" 2>&1 | head -40)
        else
            out="No UPS units found via NUT (upsc -l returned empty)."
        fi
    elif command -v apcaccess &>/dev/null; then
        out=$(apcaccess 2>&1 | head -40)
    else
        out="No UPS tools found.\nInstall NUT (nut) or apcupsd."
    fi
    dialog --title "UPS Status" --msgbox "$out" 25 70
}

view_logs() {
    local source
    source=$(dialog --title "View Logs" \
        --menu "Select log source:" 14 55 5 \
        1 "dplaned - last 100 lines" \
        2 "dplaned - errors only" \
        3 "nixos-rebuild - last 50 lines" \
        4 "Kernel errors" \
        5 "All services - last 5 minutes" \
        3>&1 1>&2 2>&3) || return

    local out
    case "$source" in
        1) out=$(journalctl -n 100 --no-pager -u dplaned 2>&1) ;;
        2) out=$(journalctl --no-pager -p err -u dplaned 2>&1) ;;
        3) out=$(journalctl -n 50 --no-pager -u nixos-rebuild 2>&1) ;;
        4) out=$(journalctl --no-pager -k -p err 2>&1 | head -80) ;;
        5) out=$(journalctl --no-pager --since "5 minutes ago" 2>&1 | head -100) ;;
    esac
    dialog --title "Logs" --msgbox "$out" 35 100
}

run_diagnostics() {
    local svc_out=""
    for svc in dplaned postgresql smbd nfsd; do
        local status
        status=$(systemctl is-active "$svc" 2>/dev/null || echo "not-found")
        svc_out+="  $svc: $status\n"
    done

    local zfs_out
    zfs_out=$(zpool list -H -o name,health,size,alloc,free 2>/dev/null || echo "  ZFS not available")

    local disk_out
    disk_out=$(df -h / /persist /var 2>/dev/null | awk 'NR==1 || /^\// {print "  "$0}')

    dialog --title "Diagnostics" --msgbox \
"DPlaneOS Diagnostics
====================

Services:
$(printf "%b" "$svc_out")
ZFS Pools:
$zfs_out

Disk Space:
$disk_out

Memory:
$(free -h | head -2 | awk '{print "  "$0}')

Load:
  $(uptime)" 35 80
}

emergency_shutdown() {
    local action
    action=$(dialog --title "Emergency Shutdown" \
        --menu "Select action:" 10 55 3 \
        1 "Power off system" \
        2 "Reboot system" \
        3 "Cancel" \
        3>&1 1>&2 2>&3) || return

    case "$action" in
        1)
            dialog --title "Confirm" --yesno "Power off the system NOW?" 6 45 \
                && systemctl poweroff
            ;;
        2)
            dialog --title "Confirm" --yesno "Reboot the system NOW?" 6 45 \
                && systemctl reboot
            ;;
    esac
}

# ── Entrypoint ──────────────────────────────────────────────────────────────────
show_main_menu
