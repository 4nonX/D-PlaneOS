-- +goose Up

-- TLS certificate pinning for Redfish BMC connections.
-- BMCs ship with self-signed certificates. Rather than disabling TLS
-- verification entirely, DPlane uses TOFU (Trust On First Use) pinning:
-- the SHA-256 fingerprint of the BMC's leaf certificate is captured on
-- first connection and stored here. Subsequent connections verify the
-- fingerprint matches. A changed fingerprint (MITM or BMC cert regenerated
-- after firmware update) is rejected with a clear error message.

ALTER TABLE ha_fencing_config
    ADD COLUMN IF NOT EXISTS bmc_tls_fingerprint TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bmc_tls_pinned_at   TIMESTAMPTZ;

-- bmc_protocol tracks which protocol was detected during the last probe.
-- Values: 'auto' (detect each time), 'redfish', 'ilo4', 'ipmi'.
-- Default 'auto' preserves existing behaviour for all existing installs.
ALTER TABLE ha_fencing_config
    ADD COLUMN IF NOT EXISTS bmc_protocol TEXT NOT NULL DEFAULT 'auto';

-- +goose Down

ALTER TABLE ha_fencing_config
    DROP COLUMN IF EXISTS bmc_tls_fingerprint,
    DROP COLUMN IF EXISTS bmc_tls_pinned_at,
    DROP COLUMN IF EXISTS bmc_protocol;
