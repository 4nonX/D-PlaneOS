-- +goose Up

-- Authentication Assurance Level tracks how strongly the session was authenticated.
-- AAL1 = password only. AAL2 = password + TOTP (second factor verified).
-- Default 1 so existing sessions are not invalidated by this migration.

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS aal INTEGER NOT NULL DEFAULT 1;

-- +goose Down

ALTER TABLE sessions
    DROP COLUMN IF EXISTS aal;
