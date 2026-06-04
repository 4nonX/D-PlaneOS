# DPlaneOS Port Reference

Every port the system listens on, what listens on it, whether it is configurable, and which interface it is bound to. Use this when assigning ports to Docker containers to avoid conflicts.

The key rule: **any port in the "Always active" or "Active when enabled" sections cannot be used by a container's `-p HOST:CONTAINER` binding without first reconfiguring the system service that owns it.**

---

## Always Active

These ports are open on every running DPlaneOS node.

| Port | Proto | Interface | Service | Configurable |
|------|-------|-----------|---------|-------------|
| 22 | TCP | All | OpenSSH | Yes - Settings > Security > SSH Port |
| 80 | TCP | All | nginx (HTTP, redirects to HTTPS) | No |
| 443 | TCP | All | nginx (HTTPS, TLS terminated) | No |
| 9000 | TCP | **Loopback only** (`127.0.0.1`) | dplaned daemon API | Yes - `services.dplaneos.listenPort` in NixOS config |
| 5353 | UDP | All | Avahi mDNS (`dplaneos.local` discovery) | No |

`9000` is loopback-only. Containers binding `0.0.0.0:9000` **will conflict** with the daemon because `0.0.0.0` includes `127.0.0.1`. Use any other port for containers needing port 9000 on the host, or change the daemon port.

---

## File Sharing (active when service is enabled)

| Port | Proto | Interface | Service | Configurable |
|------|-------|-----------|---------|-------------|
| 445 | TCP | All | Samba (SMB2/3) | No |
| 139 | TCP | All | Samba (NetBIOS Session Service, legacy SMB1 clients) | No |
| 137 | UDP | All | Samba (NetBIOS Name Service) | No |
| 138 | UDP | All | Samba (NetBIOS Datagram Service) | No |
| 2049 | TCP | All | NFS (nfsd) | No |
| 2049 | UDP | All | NFS (nfsd) | No |
| 111 | TCP | All | rpcbind / portmap (NFS) | No |
| 111 | UDP | All | rpcbind / portmap (NFS) | No |
| 21 | TCP | All | vsftpd control channel (FTP/FTPS) | Yes - Settings > FTP |
| 40000-40100 | TCP | All | vsftpd passive data range (FTP/FTPS) | Yes - Settings > FTP |

Samba and NFS are **enabled by default** on all DPlaneOS installations. Their ports are always open unless the modules are explicitly disabled in `configuration.nix`.

FTP/FTPS is **disabled by default** and must be started via Settings > Sharing > FTP.

---

## Block Storage Protocols (active when targets are configured)

| Port | Proto | Interface | Service | Configurable |
|------|-------|-----------|---------|-------------|
| 3260 | TCP | All | iSCSI (kernel LIO via nvmet) | No |
| 4420 | TCP | All | NVMe-oF over TCP (default per target) | Yes - per-target `listen_port` in state.yaml |

iSCSI and NVMe-oF are **off by default**. Ports only become active once a target is created in the UI or declared in `state.yaml`.

Multiple NVMe-oF targets can coexist on different ports. The default of `4420` is only used when no `listen_port` is set on a target.

---

## S3 Object Storage (MinIO, active when started)

| Port | Proto | Interface | Service | Configurable |
|------|-------|-----------|---------|-------------|
| 9000 | TCP | All | MinIO S3 API | Yes - Settings > S3 |
| 9001 | TCP | All | MinIO web console | Yes - Settings > S3 |

**Conflict warning:** MinIO defaults to `0.0.0.0:9000`, which overlaps with the daemon's `127.0.0.1:9000`. If you run MinIO, **change its API port** (e.g. to `9002`) in Settings > S3 before starting it, or change the daemon's `listenPort` in `configuration.nix`.

MinIO is **off by default** and must be started via Settings > Sharing > S3.

---

## High Availability (HA cluster nodes only)

These ports are only relevant when HA is configured. On single-node deployments none of these are open.

| Port | Proto | Interface | Service | Configurable |
|------|-------|-----------|---------|-------------|
| 2379 | TCP | All | etcd client API (Patroni DCS) | No |
| 2380 | TCP | All | etcd peer communication | No |
| 8008 | TCP | All | Patroni REST API (health checks, leader election) | No |
| 5432 | TCP | **Loopback only** | PostgreSQL direct (HAProxy checks) | No |
| 5000 | TCP | **Loopback only** | HAProxy PostgreSQL routing (daemon connects here) | No |

`5432` and `5000` are loopback-only on each node. External PostgreSQL access goes through the HA VIP and is intentionally not exposed.

Etcd (`2379`, `2380`) and Patroni (`8008`) must be reachable between both data nodes and the witness. These ports should be firewalled to the cluster management network and not exposed to clients.

---

## Reserved Loopback Ports

These ports are bound to `127.0.0.1` only. Containers using `--network=host` or any binding to `0.0.0.0` that includes the loopback range **will conflict** with these.

| Port | Service |
|------|---------|
| 9000 | dplaned daemon |
| 5432 | PostgreSQL (HA only) |
| 5000 | HAProxy (HA only) |

---

## Outbound Connections (no listening port on the NAS)

These are connections the system initiates outward. They consume no listening port on the NAS itself and do not conflict with Docker containers.

| Destination Port | Proto | Service |
|-----------------|-------|---------|
| 389 | TCP | LDAP (to directory server) |
| 636 | TCP | LDAPS (to directory server, TLS) |
| 443 | TCP | OIDC IdP, ACME/Let's Encrypt, rclone cloud targets |
| 25 / 587 / 465 | TCP | SMTP alert delivery |
| 22 | TCP | ZFS replication SSH to remote host |
| 3493 | TCP | NUT upsd (local UPS daemon, loopback) |

---

## Docker Container Port Safety

When assigning host ports to Docker containers via `-p HOST:CONTAINER` (or `ports:` in Compose), avoid:

**Always reserved (single-node and HA):**
`22`, `80`, `443`, `9000`, `5353`

**Reserved when Samba is active (default on):**
`137/UDP`, `138/UDP`, `139`, `445`

**Reserved when NFS is active (default on):**
`111`, `2049`

**Reserved when optional services are active:**
`21`, `40000-40100` (FTP), `3260` (iSCSI), `4420` (NVMe-oF), `9000-9001` (MinIO)

**Reserved on HA nodes:**
`2379`, `2380`, `5000`, `5432`, `8008`

**Safe ranges for container use on a default single-node installation:**
`1024-8079`, `8081-8999`, `9002-9999`, `10000-39999`, `40101-65535`

(excluding any ports manually opened in Settings > Firewall or declared in `state.yaml` under `system.firewall`)
