-- +goose Up

-- Network quorum witness: a neutral IP or URL (VPS, cloud endpoint, DNS server,
-- etc.) that both HA nodes independently probe to detect network isolation.
--
-- The fencing decision logic:
--   local can reach witness AND peer cannot  -> peer is isolated, safe to promote
--   neither can reach witness               -> full partition, do NOT promote
--   only peer can reach witness             -> local is isolated, do NOT promote
--   both can reach witness                  -> not a network isolation scenario,
--                                              use other fencing (IPMI/PDU)
--
-- No software needs to be installed on the witness target. It just needs to
-- respond to ICMP pings or return any HTTP response. A cloud provider's
-- metadata endpoint, a DNS nameserver, or a rented VPS all work equally well.

CREATE TABLE IF NOT EXISTS ha_network_witness (
    id              INTEGER PRIMARY KEY CHECK (id = 1),
    enable          BOOLEAN     NOT NULL DEFAULT FALSE,
    target          TEXT        NOT NULL DEFAULT '',   -- IP address or URL
    method          TEXT        NOT NULL DEFAULT 'icmp' CHECK (method IN ('icmp','http','https')),
    timeout_ms      INTEGER     NOT NULL DEFAULT 2000, -- per-probe timeout
    count           INTEGER     NOT NULL DEFAULT 3,    -- probes before deciding unreachable
    description     TEXT        NOT NULL DEFAULT ''    -- human label, e.g. "Hetzner VPS Frankfurt"
);

INSERT INTO ha_network_witness (id) VALUES (1)
ON CONFLICT (id) DO NOTHING;

-- +goose Down

DROP TABLE IF EXISTS ha_network_witness;
