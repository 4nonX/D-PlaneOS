# 🛡️ D-PlaneOS Security & Architecture (v1.14.0)

## Security Model: Command Execution

D-PlaneOS uses a multi-layered validation approach to execute system commands. This architecture is designed to minimize the risk of command injection while maintaining the flexibility needed for ZFS and Docker management.

### Active Protection in `execCommand()`
The backend includes real-time security validation for every system call:

* **Token Validation:** All command tokens are extracted and validated against the pattern: `^[a-zA-Z0-9_\/-]+$`.
* **Injection Pattern Detection:** * Blocks shell operators: `&&`, `||`, `;`, `|`
    * Blocks code execution: `` ` ``, `$`
    * Blocks redirection: `>`, `<`
    * Blocks control characters: newlines, carriage returns.
* **Security Logging:** All blocked attempts are logged with command snippets for incident response and analysis.

---

## Architecture & Trust Boundaries

### System Layers
1.  **Web UI (HTML/JS):** Client-side validation and state management.
2.  **REST API (PHP-FPM):** Session authentication, CSRF protection, and input sanitization.
3.  **Command Broker (PHP):** Validates requests against an internal whitelist of approved system operations.
4.  **System Layer:** Execution via `sudo` with a strictly defined scope in `/etc/sudoers.d/dplaneos`.

### Authentication
* **Session-based:** 30-minute inactivity timeout.
* **Multi-User Support:** v1.14.0 supports full user management with `bcrypt` password hashing.
* **Database:** SQLite with strict file permissions and automatic integrity checks on startup.

---

## Recovery Procedures (Admin Playbook)

### Lost Admin Password
To reset the admin password to default (`admin`), run this command on the host terminal:
```bash
sqlite3 /var/www/html/backend/data/dplane.db "UPDATE users SET password='\$2y$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi' WHERE username='admin'"
Database Repair
The installer includes a repair mode to fix permissions and database structures:

Bash

cd dplaneos-v1.14.0
sudo bash install.sh --repair
Known Limitations
No Built-in TLS: D-PlaneOS currently serves over HTTP. For production environments or remote access, we strictly recommend using a reverse proxy (e.g., Nginx, Caddy, or Traefik) for TLS termination.

API Tokens: Currently, the API is session-only. Programmatic API token support is planned for v1.15.0.

Reporting Vulnerabilities
If you discover a security vulnerability, please open a GitHub Issue with the [Security] label or use the GitHub Private Vulnerability Reporting feature.

Thank you for helping keep D-PlaneOS secure!

### Command Execution (v1.3.1)

**Active Protection in execCommand():**

The `execCommand()` function now includes real-time security validation:

1. **Token Validation**
   - Extracts all tokens from commands
   - Validates against pattern: `^[a-zA-Z0-9_\/-]+$`
   - Skips known safe command names
   - Blocks suspicious tokens

2. **Injection Pattern Detection**
   - Blocks shell operators: `&&`, `||`, `;`, `|`
   - Blocks code execution: `` ` ``, `$`
   - Blocks redirection: `>`, `<`
   - Blocks control characters: newlines, carriage returns

3. **Security Logging**
   - Logs all blocked attempts
   - Includes command snippet for analysis
   - Enables incident response

**Example Protection:**
```php
// This will be BLOCKED:
execCommand('sudo zpool create tank; rm -rf /', $output);
// Error: "Command contains dangerous characters"

// This will be BLOCKED:
execCommand('sudo zpool create $(whoami)', $output);
// Error: "Command contains dangerous characters"

// This will WORK:
execCommand('sudo zpool create tank /dev/sdb', $output);
// Validated and executed safely
```

### Command Broker Infrastructure

The system includes a Command Broker framework (`includes/command-broker.php`) for future enhancements:
- Whitelist of approved commands
- Type-safe parameter validation
- Currently available but not required
- Optional `execSecure()` function for stricter control

### Authentication

- Session-based authentication
- 30-minute timeout
- bcrypt password hashing
- Audit logging of all actions

### Database Security

- SQLite with proper file permissions
- Read-only fallback mode on corruption
- Integrity checks on startup
- Automatic backup before upgrades

### API Security

- CSRF protection (session-based)
- Rate limiting
- Input validation on all endpoints
- Audit trail for all mutations

## Architecture

### System Layers

```
┌─────────────────────────────────┐
│         Web UI (HTML/JS)        │
├─────────────────────────────────┤
│      REST API (PHP-FPM)         │
├─────────────────────────────────┤
│     Command Broker (PHP)        │
├─────────────────────────────────┤
│   System Commands (ZFS/Docker)  │
└─────────────────────────────────┘
```

### Trust Boundaries

1. **User → Web UI**: Session authentication
2. **Web UI → API**: CSRF tokens, rate limiting
3. **API → Command Broker**: Whitelist validation
4. **Command Broker → System**: Sudoers, parameter sanitization

### Data Flow

```
User Input
    ↓ (validation)
API Endpoint
    ↓ (authentication check)
Command Broker
    ↓ (whitelist check + parameter validation)
Sudoers
    ↓ (specific command permissions)
System Command
```

## API Versioning

**Current: v1**

All APIs are available at both:
- `/api/storage/pools.php` (legacy, maintained for compatibility)
- `/api/v1/storage/pools.php` (versioned, recommended for new integrations)

Future versions will introduce `/api/v2/` while maintaining v1 compatibility.

## Known Limitations

### High Priority

None (all critical issues addressed in v1.3.0)

### Medium Priority

1. **SQLite Write Contention**
   - Risk: Multiple simultaneous writes may cause delays
   - Mitigation: Read-only fallback mode, automatic retry logic
   - Future: Consider PostgreSQL for high-concurrency deployments

2. **No Remote API Authentication**
   - Current: Session-based, web-only
   - Future: API tokens for programmatic access

### Low Priority

1. **No Built-in TLS**
   - Use reverse proxy (nginx/caddy) for TLS termination
   - Example configurations available in docs

2. **Single User System**
   - Database schema supports multiple users
   - UI for user management not yet implemented

## Recovery Procedures

### Database Corruption

System automatically:
1. Detects corruption on startup
2. Enters read-only mode
3. Prevents further writes
4. Displays error to user

Manual recovery:
```bash
# Restore from backup
cp /var/dplane/backups/dplane-TIMESTAMP.db /var/dplane/database/dplane.db
chown www-data:www-data /var/dplane/database/dplane.db
systemctl restart php8.2-fpm
```

### System Repair

```bash
# Re-run installer in repair mode
cd dplaneos-v1.3
sudo bash install.sh
# Select option 2 (Repair)
```

### Lost Admin Password

```bash
# Reset to admin/admin
sqlite3 /var/dplane/database/dplane.db "UPDATE users SET password='$2y$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi' WHERE username='admin'"
```

## Threat Model

### In Scope

- Web-based attacks (XSS, CSRF, SQL injection)
- Command injection
- Privilege escalation
- Data integrity

### Out of Scope

- Network perimeter security (use firewall)
- Physical access (secure your hardware)
- Side-channel attacks
- DDoS mitigation (use reverse proxy)

### Attack Surfaces

1. **Web Interface**
   - Protected by: Session auth, CSRF tokens, input validation
   - Exposure: HTTP/HTTPS port

2. **API Endpoints**
   - Protected by: Authentication, rate limiting, command broker
   - Exposure: Same as web interface

3. **System Commands**
   - Protected by: Command whitelist, parameter validation, sudoers
   - Exposure: Internal only (www-data user)

### Assumptions

- Attacker has network access to web interface
- Attacker does NOT have:
  - SSH access
  - Root access
  - Physical access
  - Access to other containers on system

## Audit Trail

All actions are logged to `audit_log` table with:
- User ID
- Action type
- Resource type and name
- Timestamp
- IP address
- Additional details (JSON)

Access audit log:
```bash
sqlite3 /var/dplane/database/dplane.db "SELECT * FROM audit_log ORDER BY timestamp DESC LIMIT 50"
```

## Security Contact

Report security issues by creating a GitHub issue with the `security` label.

Do not publicly disclose security vulnerabilities until they are addressed.

## Changelog

### v1.5.0 (2026-01-28) - Enterprise Security
- **ACTIVE:** Enhanced privilege separation with sudoers
- **ACTIVE:** Least-privilege command execution
- **ACTIVE:** Explicit command whitelist per operation
- **ADDED:** Comprehensive threat model documentation
- **ADDED:** Administrator recovery playbook
- Database integrity checks on startup
- Read-only fallback for corrupted databases
- API versioning structure (/api/v1/)
- Enhanced installer with upgrade/repair modes

### v1.3.1 (2026-01-28) - Security Implementation
- **ACTIVE:** Command injection protection in execCommand()
- **ACTIVE:** Real-time token validation
- **ACTIVE:** Injection pattern detection
- **ACTIVE:** Security event logging
- Database integrity checks on startup
- Read-only fallback for corrupted databases
- API versioning structure (/api/v1/)
- Enhanced installer with upgrade/repair modes

### v1.3.0 (2026-01-27) - Security Infrastructure
- Added command broker framework (infrastructure only)
- Implemented database integrity checks
- Added read-only fallback mode
- Introduced API versioning
- Enhanced audit logging
- Improved installer with upgrade/repair modes

### v1.2.x
- Basic security controls
- Session-based authentication
- Input validation

### v1.1.x
- Initial release
- Minimal security controls
