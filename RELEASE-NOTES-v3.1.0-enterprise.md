# D-PlaneOS v3.1.0 Enterprise

Release date: 2026-02-21
Base: v3.0.0 (full codebase); superseded by v3.2.0

## Phase 1 — Production Safety Net

- **SSH Hardening**: PasswordAuthentication=false, PermitRootLogin=no; new `sshKeys` NixOS module option
- **Support Bundle**: `POST /api/system/support-bundle` — streams diagnostic .tar.gz (ZFS, SMART, journal, audit tail)
- **Pre-Upgrade ZFS Snapshots**: automatic `@pre-upgrade-<timestamp>` on all pools before every `nixos-rebuild switch`; `GET /api/nixos/pre-upgrade-snapshots`
- **Webhook Alerting**: generic HTTP webhooks for all system events; `GET/POST/DELETE /api/alerts/webhooks`, test endpoint
- **Audit HMAC Chain**: tamper-evident audit log with HMAC-SHA256 chain; `GET /api/system/audit/verify-chain`; audit.key at `/var/lib/dplaneos/audit.key`

## Phase 2 — Monitoring & Real-Time Alerting

- **Background Monitor**: debounced alerting (5min cooldown, 30s hysteresis) for inotify, ZFS health, capacity
- **WebSocket Hub**: real-time events at `WS /api/ws/monitor`
- **ZFS Pool Heartbeat**: active I/O test every 30s; auto-stops Docker on pool failure
- **Capacity Guardian**: configurable thresholds, emergency reserve dataset, auto-release at 95%+
- **Deep ZFS Health**: per-disk risk scoring, SMART JSON integration, `GET /api/zfs/health`
- **SMTP Alerting**: configurable SMTP for system alerts

## Phase 3 — GitOps Differentiator

- **Declarative state.yaml**: schema for pools, datasets, shares with stdlib YAML parser
- **By-ID Enforcement**: `/dev/disk/by-id/` required for all disk references; `/dev/sdX` rejected at parse time
- **Diff Engine**: CREATE/MODIFY/DELETE/BLOCKED/NOP classification with risk levels
- **Safety Contract**: pool destroy always BLOCKED; dataset destroy blocked if used > 0 bytes; share remove blocked if active SMB connections
- **Transactional Apply**: halts on unapproved BLOCKED items; idempotent operations
- **Drift Detection**: background worker every 5min; broadcasts `gitops.drift` WebSocket event
- **API**: `GET /api/gitops/plan`, `POST /api/gitops/apply`, `POST /api/gitops/approve`, `GET/PUT /api/gitops/state`

## Phase 4 — Appliance Hardening

- **A/B Partition Layout** (disko.nix): EFI + system-a (8G) + system-b (8G) + persist (remaining)
- **OTA Update Flow** (ota-update.sh): Ed25519 signature verification, A/B slot switch, 90s auto-revert health check
- **NixOS OTA Module** (ota-module.nix): systemd health check timer, daemon integration
- **Version Pinning** (flake.nix): kernel 6.6 LTS + OpenZFS 2.2, eval-time assertions
- **Impermanence Layer** (impermanence.nix): ephemeral root, all state persisted to /persist

## New API Routes (all phases)

```
POST   /api/system/support-bundle
GET    /api/nixos/pre-upgrade-snapshots
GET    /api/system/audit/verify-chain
GET    /api/alerts/webhooks
POST   /api/alerts/webhooks
DELETE /api/alerts/webhooks/{id}
POST   /api/alerts/webhooks/{id}/test
GET    /api/gitops/status
GET    /api/gitops/plan
POST   /api/gitops/apply
POST   /api/gitops/approve
POST   /api/gitops/check
GET    /api/gitops/state
PUT    /api/gitops/state
WS     /api/ws/monitor
GET    /api/zfs/health
GET    /api/zfs/iostat
GET    /api/zfs/events
GET    /api/zfs/capacity
POST   /api/zfs/capacity/reserve
POST   /api/zfs/capacity/release
```

## Build

```bash
cd daemon && go mod vendor && go build ./...
```

go build and go vet both pass with zero errors. All 16 gitops tests pass.
