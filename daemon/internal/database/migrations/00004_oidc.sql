-- +goose Up

-- OIDC relying-party configuration (singleton row, mirrors ldap_config pattern)
CREATE TABLE IF NOT EXISTS oidc_config (
    id              BIGINT PRIMARY KEY DEFAULT 1,
    enabled         INTEGER NOT NULL DEFAULT 0,
    issuer          TEXT NOT NULL DEFAULT '',
    client_id       TEXT NOT NULL DEFAULT '',
    client_secret   TEXT NOT NULL DEFAULT '',
    scopes          TEXT NOT NULL DEFAULT 'openid email profile',
    allowed_algs    TEXT NOT NULL DEFAULT 'RS256',
    button_label    TEXT NOT NULL DEFAULT 'Sign in with SSO',
    auto_provision  INTEGER NOT NULL DEFAULT 1,
    default_role_id BIGINT,
    group_claim     TEXT NOT NULL DEFAULT 'groups',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Links an IdP (issuer, subject) pair to a local user account.
-- subject is the stable identifier from the IdP; email is informational only.
CREATE TABLE IF NOT EXISTS oidc_identities (
    id         BIGSERIAL PRIMARY KEY,
    issuer     TEXT NOT NULL,
    subject    TEXT NOT NULL,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email      TEXT NOT NULL DEFAULT '',
    linked_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login TIMESTAMPTZ,
    UNIQUE(issuer, subject)
);

CREATE INDEX IF NOT EXISTS idx_oidc_identities_user ON oidc_identities(user_id);

-- Short-lived rows for in-flight authorization requests (state, nonce, PKCE).
-- The handler deletes each row atomically via DELETE...RETURNING so it cannot
-- be replayed.
CREATE TABLE IF NOT EXISTS oidc_state (
    id            BIGSERIAL PRIMARY KEY,
    state         TEXT NOT NULL UNIQUE,
    nonce         TEXT NOT NULL,
    pkce_verifier TEXT NOT NULL,
    redirect_uri  TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '10 minutes'
);

CREATE INDEX IF NOT EXISTS idx_oidc_state_state   ON oidc_state(state);
CREATE INDEX IF NOT EXISTS idx_oidc_state_expires ON oidc_state(expires_at);

-- One-time codes for the SPA to exchange for its session after the IdP
-- redirect. Necessary because the session token cannot ride the redirect URL
-- (history/log exposure) and there is no Set-Cookie path in this header-based
-- session model.
CREATE TABLE IF NOT EXISTS oidc_handoff (
    id         BIGSERIAL PRIMARY KEY,
    code       TEXT NOT NULL UNIQUE,
    session_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '2 minutes'
);

CREATE INDEX IF NOT EXISTS idx_oidc_handoff_code    ON oidc_handoff(code);
CREATE INDEX IF NOT EXISTS idx_oidc_handoff_expires ON oidc_handoff(expires_at);

-- +goose Down
DROP TABLE IF EXISTS oidc_handoff;
DROP TABLE IF EXISTS oidc_state;
DROP TABLE IF EXISTS oidc_identities;
DROP TABLE IF EXISTS oidc_config;
