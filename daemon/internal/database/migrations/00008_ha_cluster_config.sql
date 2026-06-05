-- +goose Up

-- Runtime-configurable HA cluster timing parameters.
-- These values were previously compile-time constants in ha/cluster.go.
-- Moving them to the database allows per-deployment tuning without recompiling.
--
-- failover_after_seconds: how long a peer must be silent before promotion is
--   considered. Default 45s (3 missed heartbeats at 15s interval). Increase
--   for high-latency WAN links; decrease for low-latency LAN with tight SLAs.
--
-- hysteresis_window_minutes: minimum time after a failover before another
--   auto-failover is permitted. Default 60 minutes. Prevents ping-pong
--   flapping on an unstable network.
--
-- heartbeat_interval_seconds: how often each node pings its peers.
--   Default 15s. Must satisfy: heartbeat_interval * 3 <= failover_after.

CREATE TABLE IF NOT EXISTS ha_cluster_config (
    id                         INTEGER PRIMARY KEY CHECK (id = 1),
    failover_after_seconds     INTEGER NOT NULL DEFAULT 45,
    hysteresis_window_minutes  INTEGER NOT NULL DEFAULT 60,
    heartbeat_interval_seconds INTEGER NOT NULL DEFAULT 15
);

INSERT INTO ha_cluster_config (id) VALUES (1) ON CONFLICT DO NOTHING;

-- +goose Down

DROP TABLE IF EXISTS ha_cluster_config;
