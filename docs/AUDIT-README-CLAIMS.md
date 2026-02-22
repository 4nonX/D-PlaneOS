# Audit: README and Project Claims vs Codebase

This document records the results of auditing the repository against the goals and claims in the README and related docs. Corrections have been applied where the README was out of date.

## Summary of Corrections Applied

| Claim (before) | Reality | Action |
|----------------|---------|--------|
| Go 1.22+ | `daemon/go.mod` specifies `go 1.21` | README updated to "Go 1.21+" |
| OOM 1 GB limit | `systemd/dplaned.service` has `MemoryMax=512M` | README updated to "512MB limit" |
| 34 permissions | `schema.go` seeds 28 permissions | README updated to "28 permissions" |
| 171 API routes | 244 `r.Handle`/`r.HandleFunc` registrations in `main.go` | README updated to "170+ API routes" |
| LDAP-REFERENCE.md | File does not exist | Removed from Documentation list |

## Verified Claims (No Change)

- **Material Design 3:** App uses `m3-tokens.css`, `m3-components.css`, `material-symbols`, `material-theme.css`, etc.
- **ZFS, Docker, RBAC, LDAP/AD:** Implemented; LDAP client uses TLS 1.2+ (`MinVersion: tls.VersionTLS12` in `daemon/internal/ldap/client.go`).
- **LDAP presets:** Active Directory, OpenLDAP, FreeIPA, Custom in `app/pages/directory-service.html`.
- **JIT provisioning, group→role mapping, background sync:** Present in schema and handlers (`jit_provisioning`, `default_role`, `sync_interval`, etc.).
- **SQLite:** WAL, `synchronous=FULL`, daily backup (and optional off-pool path) confirmed in `main.go`.
- **Command whitelist:** `daemon/internal/security/whitelist.go` and `ValidateCommand`; injection tests in CI.
- **Safe container update, Time Machine, ephemeral sandbox, ZFS health, NixOS guard, replication:** Routes and behavior present in `main.go` and related handlers.
- **ZED hook:** `zed/dplaneos-notify.sh` and docs reference it.
- **Docs present:** `ADMIN-GUIDE.md`, `ERROR-REFERENCE.md`, `TROUBLESHOOTING.md`, `INSTALLATION-GUIDE.md`, `nixos/README.md`, `CHANGELOG.md`.

## Notes

- **Route count:** The daemon registers 244 route handlers in `main.go`; "170+ API routes" is a conservative description that stays accurate as routes are added or reorganized.
- **Permissions:** The live schema is seeded from `daemon/cmd/dplaned/schema.go` (28 permissions). The file `daemon/internal/database/migrations/008_rbac_system.sql` defines more permissions (34+); that migration may be used in other deployment paths. The README now reflects the 28 permissions actually seeded by the daemon.
- **LDAP:** Configuration is documented in the README under "LDAP / Active Directory" and in the UI (Identity → Directory Service). A separate `LDAP-REFERENCE.md` can be added later if a technical reference is desired.
