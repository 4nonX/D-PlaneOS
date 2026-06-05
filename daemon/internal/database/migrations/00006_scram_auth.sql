-- +goose Up

-- SCRAM-SHA-512 credential storage (RFC 5802).
-- scram_stored_key = H(ClientKey)  - verifies client proof without knowing the password.
-- scram_server_key = HMAC(SaltedPassword, "Server Key") - proves server identity to client.
-- scram_salt / scram_iterations are required to recompute the auth-message on the server side.
-- bcrypt password_hash is kept for backward compatibility during the transition window;
-- once all users have logged in and triggered SCRAM key derivation it will be cleared.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS scram_salt        TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS scram_iterations  INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS scram_stored_key  TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS scram_server_key  TEXT    NOT NULL DEFAULT '';

-- Configurable fault tolerance for SCSI-3 PR disk fencing.
-- ha_fencing_config is normally created by ensureSchema() in cluster.go at
-- daemon startup. On a clean install where this migration runs before the first
-- daemon start, we create the table here so the ALTER below has something to
-- modify. IF NOT EXISTS makes this a no-op on any existing install.

CREATE TABLE IF NOT EXISTS ha_fencing_config (
    id                       INTEGER PRIMARY KEY CHECK (id = 1),
    enable                   BOOLEAN NOT NULL DEFAULT FALSE,
    bmc_ip                   TEXT    NOT NULL DEFAULT '',
    bmc_user                 TEXT    NOT NULL DEFAULT '',
    bmc_password_file        TEXT    NOT NULL DEFAULT '',
    jitter_max_ms            INTEGER NOT NULL DEFAULT 3000,
    disk_fault_tolerance_pct INTEGER NOT NULL DEFAULT 10
);

-- Add column on any existing install where ensureSchema() already created the
-- table without this column. IF NOT EXISTS makes this safe to re-run.
ALTER TABLE ha_fencing_config
    ADD COLUMN IF NOT EXISTS disk_fault_tolerance_pct INTEGER NOT NULL DEFAULT 10;

-- +goose Down

ALTER TABLE users
    DROP COLUMN IF EXISTS scram_salt,
    DROP COLUMN IF EXISTS scram_iterations,
    DROP COLUMN IF EXISTS scram_stored_key,
    DROP COLUMN IF EXISTS scram_server_key;

ALTER TABLE ha_fencing_config
    DROP COLUMN IF EXISTS disk_fault_tolerance_pct;
