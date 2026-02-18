# D-PlaneOS v2.2.0 — API Error Reference (Exhaustive)

**The definitive reference for HTTP status codes, validation patterns, and system-level exceptions.**

---

## HTTP Status Codes

| Code | Meaning | v2.2.0 Trigger |
|------|---------|----------------|
| 200 | OK | Request succeeded. |
| 400 | Bad Request | Logic error, validation failure, or Nix syntax error. |
| 401 | Unauthorized | Session expired or missing `X-Session-ID`. |
| 403 | Forbidden | RBAC check failed. |
| 404 | Not Found | Resource (Dataset/Container/Generation) missing. |
| 409 | Conflict | **GitOps:** Remote diverged. **ZFS:** Name collision. |
| 423 | Locked | **Boot-Gate:** Pool is not imported/mounted. |
| 500 | Internal Error | Daemon crash or binary (zfs/git/nix) execution failure. |

---

## Input Validation & Guardrails (400)

### 1. GitOps & Remote State
| Error Message | Cause | Valid Format |
|---------------|-------|-------------|
| `GitOps: Invalid Remote URL` | Non-standard URL format. | `git@domain.com:user/repo.git` |
| `GitOps: SSH Auth Failed` | Key rejected by remote. | Ed25519/RSA-4096 (no passphrase). |
| `GitOps: Sync Timeout` | Upstream unresponsive. | Check network/firewall. |

### 2. NixOS & Flake Management
| Error Message | Cause | Resolution |
|---------------|-------|------------|
| `Nix: Syntax Error` | `flake.nix` fails parse. | Use `nix flake check` via CLI. |
| `Nix: Input Mismatch` | Input/Output lock mismatch. | Run `nix flake update` in the UI. |
| `Nix: Generation Locked` | System is currently activating. | Wait for previous task to finish. |

### 3. ZFS & Storage (Hardened)
| Error Message | Cause | Valid Format |
|---------------|-------|-------------|
| `Invalid dataset name` | Starts with number/symbol. | `^[a-zA-Z][a-zA-Z0-9_\-\.\/]{0,254}$` |
| `Invalid quota value` | Format not G, T, or M. | `500G`, `1T`, `100M`. |
| `Encryption: Key missing` | Attempt to unlock without key. | Provide Base64 key or passphrase. |

---

## RBAC & Permission Matrix (403)

In v2.2.0, the permission count increased to **38** to accommodate the new system-level state controls.

| Permission | Description | Fix |
|------------|-------------|-----|
| `system:gitops:sync` | Trigger manual push/pull. | Assign `admin` or custom GitOps role. |
| `system:nixos:edit` | Modify the system Flake. | Requires `admin` privileges. |
| `system:nixos:boot` | Switch/Rollback generations. | Requires `admin` privileges. |
| `docker:safe-update` | Trigger ZFS-Atomic updates. | Requires `operator` or higher. |

---

## ZFS & System Operation Exceptions (500)

These patterns appear in the `error` field of the JSON response when a system binary exits with a non-zero status.

### ZFS Errors
- `dataset is busy`: Snapshot or clone in use by a container or share.
- `out of space`: Pool has reached 100% (ZFS-Atomic updates will fail).
- `pool is suspended`: Hardware I/O failure. Check cables/controller.

### v2.2.0 State Errors
- `Boot-Gate: Dependency timeout`: The system waited 60s for ZFS but it never mounted. Services remain 423 Locked.
- `State: Hash Mismatch`: Local state differs from the Git-signed hash. Indicates manual tampering.

---

## Diagnostic & Audit Logging

The Go-daemon (v2.2.0) logs everything to the journal.



### Real-time Debugging
```bash
# Filter for GitOps failures
journalctl -u dplaned -g "GITOPS" --since "1 hour ago"

# Monitor NixOS activation output
journalctl -u dplaned -g "NIX" -f

# Check for RBAC denials (Security Auditing)
journalctl -u dplaned -g "FORBIDDEN"
```

### v2.2.0 Health Endpoints
- `GET /health` — Simple 200/500 check.
- `GET /health/storage` — Returns 423 if Boot-Gate is still active.
- `GET /health/git` — Returns status of the last sync operation.

---

**Next Steps:**
- See `GITOPS-WORKFLOW.md` for conflict resolution strategies.
- See `NIXOS-FLAKE.md` for the base v2.2.0 template.
