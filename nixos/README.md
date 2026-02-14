# D-PlaneOS auf NixOS

> Dein komplettes NAS, definiert in einer einzigen Textdatei. Reproduzierbar, versioniert, unzerstörbar.

## Zwei Installationswege

### Weg 1: Flake (empfohlen)

Reproduzierbar, versioniert, ein Befehl zum Updaten.

```bash
# Auf einem laufenden NixOS:
git clone https://github.com/4nonX/D-PlaneOS /tmp/dplaneos
cd /tmp/dplaneos/nixos

# Setup-Helper ausführen (füllt Host-ID, Zeitzone etc. aus)
sudo bash setup-nixos.sh

# System bauen
sudo nixos-rebuild switch --flake .#dplaneos

# Browser öffnen
# → http://dplaneos.local
```

**Update:**
```bash
cd /tmp/dplaneos/nixos && git pull
sudo nixos-rebuild switch --flake .#dplaneos
```

**Rollback:**
```bash
sudo nixos-rebuild switch --rollback
```

### Weg 2: Standalone (ohne Flake)

Einfacher, wenn du Flakes noch nicht nutzen willst.

```bash
# Kopiere die standalone Config
sudo cp configuration-standalone.nix /etc/nixos/configuration.nix

# Setup-Helper
sudo bash setup-nixos.sh

# Bauen
sudo nixos-rebuild switch
```

## Dateien

| Datei | Beschreibung |
|-------|-------------|
| `flake.nix` | Flake-Definition — Packages, Inputs, System |
| `configuration.nix` | NAS-Config (Flake-Version, empfangen Packages via `specialArgs`) |
| `configuration-standalone.nix` | NAS-Config (Standalone, Packages inline definiert) |
| `setup-nixos.sh` | Setup-Helper — generiert Host-ID, prüft Bootloader, etc. |
| `NIXOS-INSTALL-GUIDE.md` | Komplette Schritt-für-Schritt Anleitung (Deutsch, für Anfänger) |
| `NIXOS-README.md` | Technische Details, Rollback, Git-Versionierung |

## Systemanforderungen

- NixOS 24.11 (stable)
- Mindestens 8 GB RAM (empfohlen: 32 GB für 16 GB ZFS ARC)
- Separate Boot-Disk (SSD) + Daten-Disks (HDD/SSD für ZFS-Pool)
- Netzwerkverbindung

## Was die Config enthält

| Komponente | Details |
|-----------|---------|
| **ZFS** | Auto-Import, monatlicher Scrub, Auto-Snapshots (15min/hourly/daily/weekly/monthly) |
| **D-PlaneOS Daemon** | systemd-Service, OOM-geschützt (1 GB), Hardened (ProtectSystem=strict) |
| **nginx** | Reverse Proxy, Security Headers, PHP blockiert |
| **Docker** | ZFS Storage-Driver, wöchentliches Prune |
| **Samba** | Performance-optimiert, dynamische Shares via Daemon |
| **NFS** | Server aktiviert |
| **S.M.A.R.T.** | Automatische Festplattenüberwachung |
| **Firewall** | Nur Ports 80, 443, 445, 2049 offen |
| **mDNS** | NAS sichtbar als `dplaneos.local` |
| **SSH** | Passwort-Login für Ersteinrichtung, danach SSH-Keys empfohlen |
| **Backups** | Tägliches SQLite-Backup um 3 Uhr (`.backup`, WAL-safe) |

## Sicherheit

- `sudo` erfordert Passwort
- Daemon läuft mit `ProtectSystem=strict`, `PrivateTmp`, Capability-Begrenzung
- SQLite-Backups nutzen `.backup` (konsistent bei WAL-Mode)
- Daemon startet erst nach `zfs-mount.service` (kein Race-Condition)
- OOM-Schutz: Daemon auf 1 GB begrenzt, OOMScoreAdjust=-900
- Nginx blockiert PHP, versteckte Dateien, sensitive Verzeichnisse

## Konfigurierbare Daemon-Flags

Der Go-Daemon unterstützt NixOS nativ über CLI-Flags:

```
dplaned \
  -config-dir /var/lib/dplaneos/config \
  -smb-conf /var/lib/dplaneos/smb-shares.conf \
  -db /var/lib/dplaneos/dplaneos.db \
  -listen 127.0.0.1:9000
```

Auf Debian werden die Defaults verwendet (`/etc/dplaneos`, `/etc/samba/smb.conf`). Selbes Binary, beide Distros.
