# D-PlaneOS v3.0.0 - Dependencies

**Complete dependency list for the Go daemon and frontend**

---

## ✅ All Dependencies Are Vendored

The package contains **ALL** necessary Go dependencies in `daemon/vendor/`. No internet access needed at build time.

---

## Runtime Dependencies

### Required
| Package | Purpose |
|---------|---------|
| `nginx` | Reverse proxy, TLS termination, static file serving |
| `zfsutils-linux` | ZFS pool/dataset/snapshot management |
| `sqlite3` | Database engine (embedded via go-sqlite3, CLI optional) |
| `gcc` | Required for CGO (go-sqlite3 build) |
| `systemd` | Service management, OOM protection |

### Optional (graceful degradation if absent)
| Package | Purpose |
|---------|---------|
| `docker-ce` / `docker.io` | Container management |
| `samba` | SMB/CIFS file sharing |
| `nfs-kernel-server` | NFS exports |
| `nut` / `nut-client` | UPS monitoring |
| `ipmitool` | IPMI/BMC hardware sensors |
| `smartmontools` | Disk S.M.A.R.T. monitoring |
| `rclone` | Cloud sync |
| `ufw` | Firewall management |

---

## Go Dependencies (Vendored)

| Module | Version | Purpose |
|--------|---------|---------|
| `github.com/mattn/go-sqlite3` | v1.14.x | SQLite database driver (CGO) |
| `github.com/gorilla/websocket` | v1.5.x | WebSocket support for live monitoring |
| `golang.org/x/crypto` | latest | bcrypt password hashing |

All Go dependencies are in `daemon/vendor/` and compiled into the single `dplaned` binary.

---

## Frontend Dependencies

The frontend uses **zero external frameworks**. All assets are self-contained:

- HTML5 + vanilla JavaScript (no React, Vue, Angular)
- Material Design 3 CSS (custom implementation)
- Material Symbols font (locally hosted, no CDN)

---

## Build Requirements

| Tool | Minimum Version | Purpose |
|------|----------------|---------|
| Go | 1.22+ | Compile daemon |
| gcc | any | CGO for go-sqlite3 |
| make | any | Build system |

### Build from source
```bash
cd daemon && make build
# Output: daemon/dplaned (single binary, ~8 MB)
```

### Pre-built binary
The release tarball includes a pre-built `dplaned` binary. `make install` uses it directly — no compiler needed for deployment.
