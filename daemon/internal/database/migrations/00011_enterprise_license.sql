-- +goose Up

-- Enterprise license management
-- License keys are Ed25519-signed payloads (base64_sig.base64_payload format)
-- Validation is offline (no external API calls) using embedded public key
-- License can specify: customer name, audit limit (or unlimited), expiration date
-- When valid, triggers automatic download and installation of Compliance Engine code

CREATE TABLE IF NOT EXISTS enterprise_license (
    id                          INTEGER PRIMARY KEY CHECK (id = 1),
    customer_name               TEXT        NOT NULL,
    audits_limit                INTEGER     NOT NULL DEFAULT -1,  -- -1 = unlimited
    audits_consumed             INTEGER     NOT NULL DEFAULT 0,
    expires_at                  TEXT        NOT NULL,              -- RFC3339 or 'never'
    license_key                 TEXT        NOT NULL,              -- Full key for renewal
    ce_repo_url                 TEXT        NOT NULL,              -- Private repo URL
    ce_version                  TEXT        NOT NULL,              -- Version downloaded
    activated_at                TIMESTAMP WITH TIME ZONE NOT NULL,
    activated_by                TEXT        NOT NULL DEFAULT 'api' -- User ID or 'api'
);

INSERT INTO enterprise_license (id, customer_name, expires_at, license_key, ce_repo_url, ce_version, activated_at)
VALUES (1, '', 'never', '', '', '', NOW())
ON CONFLICT (id) DO NOTHING;

-- Track audit report generations for compliance reporting
CREATE TABLE IF NOT EXISTS enterprise_audit_usage (
    id                          SERIAL PRIMARY KEY,
    generated_at                TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    audits_before               INTEGER     NOT NULL,
    audits_after                INTEGER     NOT NULL,
    customer_name               TEXT        NOT NULL,
    report_size_bytes           INTEGER
);

CREATE INDEX IF NOT EXISTS idx_enterprise_audit_usage_generated_at
ON enterprise_audit_usage (generated_at DESC);

-- +goose Down

DROP TABLE IF EXISTS enterprise_audit_usage;
DROP TABLE IF EXISTS enterprise_license;
