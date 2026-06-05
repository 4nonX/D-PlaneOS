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
-- When dplane-fenced starts up or refreshes its pool disk list it will attempt to
-- register and reserve every pool member disk. If a disk fails the reservation
-- and the failure count is within the tolerance window, the daemon continues
-- running rather than aborting. This mirrors TrueNAS's 10% default threshold.
-- Set to 0 to restore the previous all-or-nothing behaviour.

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
