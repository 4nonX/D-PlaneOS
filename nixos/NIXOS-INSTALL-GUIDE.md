# D-PlaneOS auf NixOS — Komplettanleitung für Einsteiger

> **Zielgruppe**: Du hast noch nie NixOS benutzt. Du willst ein NAS.
> Diese Anleitung bringt dich von "leerer Server" zu "D-PlaneOS läuft" — Schritt für Schritt, ohne Vorwissen.

---

## Was ist NixOS (30-Sekunden-Version)

NixOS ist ein Linux, bei dem das **gesamte System in einer einzigen Textdatei** definiert wird: `configuration.nix`. Du beschreibst dort alles — welche Programme installiert sind, welche Services laufen, welche Firewall-Regeln gelten. Danach sagst du `sudo nixos-rebuild switch` und NixOS baut das System exakt so, wie du es beschrieben hast.

**Warum für ein NAS?**
- Kaputtes Update? → `sudo nixos-rebuild switch --rollback` — ein Befehl und alles ist wie vorher
- Server stirbt? → NixOS auf neuer Hardware installieren, `configuration.nix` kopieren, ZFS-Pool importieren — fertig
- Dein gesamtes NAS ist in einer Datei versionierbar (Git)

---

## Was du brauchst

- Einen PC/Server für das NAS (mindestens 4 GB RAM, besser 8+)
- Einen USB-Stick (mindestens 2 GB) für den NixOS-Installer
- Eine **separate Boot-Disk** (SSD/HDD/NVMe) — NixOS wird hier installiert
- Deine Daten-Disks (werden als ZFS-Pool verwendet — **nicht** für NixOS)
- Einen zweiten Computer um diese Anleitung zu lesen und die Config zu bearbeiten
- Netzwerkkabel (WLAN geht auch, ist aber bei der Installation umständlicher)

---

## Teil 1: NixOS installieren (ca. 20 Minuten)

### Schritt 1.1 — ISO herunterladen

Geh auf **https://nixos.org/download** und lade das **Minimal ISO image** herunter (64-bit). Nicht die Graphical ISO — wir brauchen kein Desktop.

### Schritt 1.2 — USB-Stick erstellen

**Windows:** Benutze [Rufus](https://rufus.ie/) oder [balenaEtcher](https://etcher.balena.io/)
**Mac/Linux:**
```bash
# Finde deinen USB-Stick (VORSICHT: richtiges Gerät wählen!)
lsblk

# Schreibe das ISO (ersetze /dev/sdX mit deinem USB-Stick)
sudo dd if=nixos-minimal-*.iso of=/dev/sdX bs=4M status=progress
```

### Schritt 1.3 — Vom USB-Stick booten

1. USB-Stick in den NAS-Server stecken
2. Server starten, ins BIOS gehen (meist F2, F12 oder DEL beim Hochfahren)
3. Boot-Reihenfolge: USB-Stick als erstes
4. Speichern und neustarten

Du landest auf einer Kommandozeile: `[nixos@nixos:~]$` — das ist der NixOS Live-Installer.

### Schritt 1.4 — Internet prüfen

```bash
ping -c 3 google.com
```

Wenn das funktioniert → weiter. Wenn nicht:

```bash
# WLAN (falls nötig):
sudo systemctl start wpa_supplicant
wpa_cli
> add_network
> set_network 0 ssid "DeinWLANName"
> set_network 0 psk "DeinWLANPasswort"
> enable_network 0
> quit
```

### Schritt 1.5 — Boot-Disk partitionieren

**ACHTUNG: Das löscht ALLES auf der gewählten Disk. Stelle sicher, dass du die richtige Disk wählst — NICHT deine Daten-Disks!**

```bash
# Zeige alle Disks an
lsblk

# Beispiel: /dev/sda ist deine Boot-SSD (120GB)
#           /dev/sdb, /dev/sdc, /dev/sdd sind deine Daten-Disks → NICHT ANFASSEN
```

**Für UEFI-Systeme** (die meisten modernen Server/PCs seit ~2012):

```bash
# Partitionieren
sudo parted /dev/sda -- mklabel gpt
sudo parted /dev/sda -- mkpart ESP fat32 1MB 512MB
sudo parted /dev/sda -- set 1 esp on
sudo parted /dev/sda -- mkpart primary 512MB 100%

# Formatieren
sudo mkfs.fat -F 32 -n BOOT /dev/sda1
sudo mkfs.ext4 -L nixos /dev/sda2

# Mounten
sudo mount /dev/disk/by-label/nixos /mnt
sudo mkdir -p /mnt/boot
sudo mount /dev/disk/by-label/BOOT /mnt/boot
```

**Für ältere BIOS/MBR-Systeme:**

```bash
sudo parted /dev/sda -- mklabel msdos
sudo parted /dev/sda -- mkpart primary 1MB 100%

sudo mkfs.ext4 -L nixos /dev/sda1

sudo mount /dev/disk/by-label/nixos /mnt
```

### Schritt 1.6 — NixOS Grundconfig generieren

```bash
sudo nixos-generate-config --root /mnt
```

Das erstellt zwei Dateien:
- `/mnt/etc/nixos/hardware-configuration.nix` — automatisch erkannte Hardware (NIEMALS manuell bearbeiten)
- `/mnt/etc/nixos/configuration.nix` — hier kommt unsere D-PlaneOS Config rein

### Schritt 1.7 — D-PlaneOS Config einspielen

Jetzt ersetzt du die generierte `configuration.nix` mit unserer. Du hast zwei Optionen:

**Option A: Direkt auf dem Server bearbeiten:**
```bash
sudo nano /mnt/etc/nixos/configuration.nix
```
Lösche alles und kopiere den kompletten Inhalt von `configuration.nix` (die mitgelieferte Datei) hinein.

**Option B: Von einem anderen PC per USB-Stick:**
Kopiere `configuration.nix` auf einen zweiten USB-Stick, stecke ihn ein und:
```bash
# Zweiten USB-Stick finden
lsblk
sudo mount /dev/sdX1 /media

# Kopieren
sudo cp /media/configuration.nix /mnt/etc/nixos/configuration.nix
sudo umount /media
```

### Schritt 1.8 — Setup-Script ausführen

Statt die 5 Stellen manuell zu suchen, gibt es ein Script das alles für dich erledigt:

```bash
# Kopiere das Setup-Script (vom gleichen USB-Stick oder direkt)
sudo cp /media/setup-nixos.sh /mnt/root/setup-nixos.sh

# Hinweis: Das Script wird NACH dem ersten Reboot ausgeführt,
# nicht jetzt! Weiter mit Schritt 1.9.
```

Falls du das Script nicht hast, kannst du die 5 Stellen auch manuell bearbeiten:

```bash
sudo nano /mnt/etc/nixos/configuration.nix
# Suche mit Ctrl+W nach "HIER" — es gibt 5 Stellen
```

| # | Was | Wo in der Datei | Beispiel |
|---|-----|-----------------|----------|
| 1 | ZFS Pool-Name | `zpools = [ "tank" ];` | Dein Poolname, z.B. `"datapool"` |
| 2 | Host-ID | `networking.hostId = "..."` | Wird automatisch generiert (siehe unten) |
| 3 | Zeitzone | `time.timeZone = "..."` | z.B. `"Europe/Berlin"` |
| 4 | UEFI oder BIOS | Boot-Loader Sektion | Siehe unten |
| 5 | Admin-Passwort | Nach Installation | `sudo passwd admin` |

**Host-ID generieren** (muss pro Maschine einzigartig sein — ZFS braucht das):
```bash
head -c4 /dev/urandom | od -A none -t x4 | tr -d ' '
# Gibt z.B. aus: a8f3b2c1
# Diesen Wert bei networking.hostId eintragen
```

**UEFI oder BIOS?** Du hast in Schritt 1.5 entweder UEFI oder BIOS gewählt. Die Config muss dazu passen. In der Datei steht:

Für **UEFI** (der häufigste Fall):
```nix
  boot.loader.systemd-boot.enable = true;
  boot.loader.efi.canTouchEfiVariables = true;
```

Für **BIOS/MBR**:
```nix
  boot.loader.grub.enable = true;
  boot.loader.grub.device = "/dev/sda";
```

### Schritt 1.9 — Installieren!

```bash
sudo nixos-install
```

Das dauert 5-15 Minuten (je nach Internet-Geschwindigkeit). Am Ende wirst du nach einem **Root-Passwort** gefragt — wähle ein sicheres.

```bash
# Fertig! Neustart.
sudo reboot
```

**USB-Stick entfernen!** Der Server startet jetzt von der Boot-Disk in dein neues NixOS.

---

## Teil 2: D-PlaneOS einrichten (ca. 5 Minuten)

### Schritt 2.1 — Einloggen

Nach dem Neustart siehst du einen Login-Prompt. Logge dich ein:

```
Benutzer: root
Passwort: (was du bei nixos-install gewählt hast)
```

### Schritt 2.1b — Setup-Script ausführen (empfohlen)

Falls du das Setup-Script kopiert hast:

```bash
bash /root/setup-nixos.sh
```

Das Script erledigt automatisch:
- Host-ID generieren und eintragen
- Zeitzone bestätigen oder ändern
- ZFS Pool-Name prüfen
- Bootloader erkennen (UEFI/BIOS)

Danach:
```bash
sudo nixos-rebuild switch
```

**Beim ersten Mal** wird der Build wegen fehlender Paket-Hashes fehlschlagen. Das Script erklärt dir genau wie du das in 3 Minuten löst (Hash aus Fehlermeldung kopieren → eintragen → nochmal bauen).

### Schritt 2.2 — IP-Adresse herausfinden

```bash
ip addr show | grep "inet "
# Suche die Adresse die NICHT 127.0.0.1 ist
# Beispiel: 192.168.178.42
```

### Schritt 2.3 — ZFS-Pool importieren

**Wenn du einen bestehenden ZFS-Pool hast** (z.B. von einer TrueNAS/Debian Migration):
```bash
# Zeige verfügbare Pools
zpool import

# Importiere deinen Pool
zpool import tank
# (ersetze "tank" mit deinem Pool-Namen)

# Prüfe ob er da ist
zpool status
```

**Wenn du einen neuen Pool erstellen willst:**
```bash
# Zeige verfügbare Disks
lsblk

# Erstelle einen Mirror-Pool (2 Disks, empfohlen)
zpool create tank mirror /dev/sdb /dev/sdc

# ODER: RAIDZ1 (3+ Disks, eine darf ausfallen)
zpool create tank raidz1 /dev/sdb /dev/sdc /dev/sdd

# Docker-Dataset erstellen
zfs create tank/docker
```

### Schritt 2.4 — Prüfen ob alles läuft

```bash
# Daemon läuft?
systemctl status dplaned
# Sollte "active (running)" zeigen

# Nginx läuft?
systemctl status nginx
# Sollte "active (running)" zeigen

# Alle Services OK?
systemctl --failed
# Sollte leer sein
```

### Schritt 2.5 — Browser öffnen

Auf deinem normalen PC, öffne den Browser:

```
http://192.168.178.42
```
(Ersetze mit der IP aus Schritt 2.2)

Oder probiere:
```
http://dplaneos.local
```
(Funktioniert dank mDNS auf den meisten Betriebssystemen automatisch)

**Du siehst den D-PlaneOS Setup-Wizard!** Folge den Anweisungen im Browser.

---

## Teil 3: Alltag — die 5 Befehle die du brauchst

### Etwas ändern
```bash
# Config bearbeiten
sudo nano /etc/nixos/configuration.nix

# Anwenden
sudo nixos-rebuild switch
```

### Etwas kaputt gemacht?
```bash
# Zurück zum letzten funktionierenden Stand
sudo nixos-rebuild switch --rollback
```

### System updaten
```bash
# NixOS + alle Pakete aktualisieren
sudo nix-channel --update
sudo nixos-rebuild switch
```

### Server neustarten
```bash
sudo reboot
```

### Status prüfen
```bash
systemctl status dplaned    # D-PlaneOS Daemon
zpool status                 # ZFS Pools
docker ps                    # Docker Container
```

---

## Häufige Probleme

### "error: hash mismatch" bei nixos-rebuild

Die `sha256-FIXME` Hashes in der Config müssen ausgefüllt werden. Wenn der D-PlaneOS v2.0.0 Release auf GitHub getaggt ist:

```bash
# Installiere das Prefetch-Tool
nix-shell -p nix-prefetch-github

# Hole den Hash
nix-prefetch-github 4nonX dplaneos --rev v2.0.0
# → Gibt dir den sha256 Hash, den du bei "hash = " einträgst
```

Für den `vendorHash` (Go dependencies): Setze ihn erstmal auf `""` und lass `nixos-rebuild switch` laufen — die Fehlermeldung zeigt dir den korrekten Hash.

### ZFS Pool wird nicht importiert

```bash
# Manuell importieren
sudo zpool import -f tank

# Prüfen ob hostId stimmt
cat /etc/machine-id
# Muss zum Wert in configuration.nix passen
```

### "D-PlaneOS zeigt leere Seite"

```bash
# Daemon-Logs anschauen
journalctl -u dplaned -f

# Nginx-Logs anschauen
journalctl -u nginx -f
```

### SSH funktioniert nicht

Die Config erlaubt nur SSH-Key-Login. Wenn du noch keinen Key eingetragen hast:

```bash
# Temporär Passwort-Login erlauben (auf dem Server direkt):
sudo nano /etc/nixos/configuration.nix
# Ändere: PasswordAuthentication = false;
# Zu:     PasswordAuthentication = true;
sudo nixos-rebuild switch

# Jetzt von deinem PC aus:
ssh admin@dplaneos.local
# Passwort eingeben

# Dann SSH-Key einrichten und Passwort-Login wieder deaktivieren
```

### Ich will ein Paket installieren

**Nicht** `apt install` — das gibt es auf NixOS nicht. Stattdessen:

```bash
# Temporär (nur für diese Session):
nix-shell -p vim

# Permanent (überlebt Neustarts):
sudo nano /etc/nixos/configuration.nix
# Unter environment.systemPackages hinzufügen:
#   vim
# Dann:
sudo nixos-rebuild switch
```

### Samba-Shares werden nicht angezeigt

```bash
# Prüfe ob die Share-Config existiert
cat /var/lib/dplaneos/smb-shares.conf

# Prüfe Samba-Status
systemctl status smbd
testparm -s
```

---

## Für Fortgeschrittene

### Config in Git versionieren

```bash
cd /etc/nixos
sudo git init
sudo git add .
sudo git commit -m "D-PlaneOS v2.0.0 - Ersteinrichtung"

# Nach jeder Änderung:
sudo git add -A && sudo git commit -m "Beschreibung der Änderung"
sudo nixos-rebuild switch
```

### Alle Boot-Generationen anzeigen

```bash
# Jedes nixos-rebuild erstellt eine "Generation" — wie ein Snapshot
sudo nix-env --list-generations --profile /nix/var/nix/profiles/system

# Zu einer bestimmten Generation zurück:
sudo nixos-rebuild switch --rollback
# Oder beim Booten: im GRUB-Menü ältere Generation auswählen
```

### Automatische Updates (optional)

Füge in die `configuration.nix` ein:
```nix
  system.autoUpgrade = {
    enable = true;
    dates = "04:00";  # Jeden Tag um 4 Uhr morgens
    allowReboot = false;  # Kein automatischer Reboot
  };
```

---

## Cheat Sheet: NixOS vs. Debian

| Ich will... | Debian | NixOS |
|-------------|--------|-------|
| Paket installieren | `apt install vim` | In `configuration.nix` hinzufügen + `nixos-rebuild switch` |
| Service starten | `systemctl enable nginx` | `services.nginx.enable = true;` + rebuild |
| Config bearbeiten | `nano /etc/nginx/nginx.conf` | `nano /etc/nixos/configuration.nix` + rebuild |
| Update | `apt update && apt upgrade` | `nix-channel --update && nixos-rebuild switch` |
| Rollback | 😢 manuell reparieren | `nixos-rebuild switch --rollback` |
| Welche Pakete hab ich? | `dpkg -l` | Steht alles in `configuration.nix` |
| Firewall-Port öffnen | `ufw allow 8080` | `networking.firewall.allowedTCPPorts = [ 8080 ];` + rebuild |

---

## Dateien in diesem Paket

| Datei | Zweck |
|-------|-------|
| `configuration.nix` | **Die eine Datei die dein NAS definiert** — kopieren nach `/etc/nixos/` |
| `setup-nixos.sh` | **Setup-Helper** — füllt Host-ID, Zeitzone, Pool automatisch aus |
| `NIXOS-INSTALL-GUIDE.md` | Diese Anleitung |
| `NIXOS-README.md` | Technische Details für Fortgeschrittene |
