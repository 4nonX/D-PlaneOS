-- +goose Up

-- Add resource-level allowlist to API tokens.
-- allowed_resources is a JSON array of {method, resource} objects where
-- resource supports fnmatch-style wildcards (e.g. "/api/zfs/*", "/api/shares/*").
-- An empty array means "inherit from the coarse scope (read/write/admin)".
-- A non-empty array further restricts the token to only those resource patterns.
--
-- Examples:
--   [{"method":"GET","resource":"/api/zfs/*"}]
--       -> read-only access to all ZFS endpoints
--   [{"method":"*","resource":"/api/gitops/*"},{"method":"GET","resource":"/api/ha/status"}]
--       -> full GitOps access plus read-only HA status
--
-- This mirrors TrueNAS's per-API-key allowlist (utils/allowlist.py).
ALTER TABLE api_tokens ADD COLUMN IF NOT EXISTS allowed_resources TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE api_tokens DROP COLUMN IF EXISTS allowed_resources;
