package ha

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// NetworkWitnessConfig holds the configuration for the network quorum witness.
type NetworkWitnessConfig struct {
	Enable      bool   `json:"enable"`
	Target      string `json:"target"`      // IP address or URL
	Method      string `json:"method"`      // "icmp", "http", "https"
	TimeoutMs   int    `json:"timeout_ms"`
	Count       int    `json:"count"`       // probes before deciding unreachable
	Description string `json:"description"` // human label
}

// NetworkWitnessResult is the outcome of probing the network witness.
type NetworkWitnessResult struct {
	Reachable bool   `json:"reachable"`
	Latency   int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ProbeNetworkWitness checks whether the configured witness target is reachable.
// This is intentionally simple: it does not require any software on the target,
// just that the target responds to ICMP pings or HTTP requests.
func ProbeNetworkWitness(cfg NetworkWitnessConfig) NetworkWitnessResult {
	if !cfg.Enable || cfg.Target == "" {
		return NetworkWitnessResult{Reachable: false, Error: "network witness not configured"}
	}
	if cfg.TimeoutMs <= 0 {
		cfg.TimeoutMs = 2000
	}
	if cfg.Count <= 0 {
		cfg.Count = 3
	}

	timeout := time.Duration(cfg.TimeoutMs) * time.Millisecond
	successes := 0
	var lastErr error
	var totalLatency int64

	for i := 0; i < cfg.Count; i++ {
		start := time.Now()
		var err error
		switch cfg.Method {
		case "http", "https":
			err = probeHTTP(cfg.Target, timeout)
		default: // icmp
			err = probeICMP(cfg.Target, timeout)
		}
		latency := time.Since(start).Milliseconds()
		if err == nil {
			successes++
			totalLatency += latency
		} else {
			lastErr = err
		}
	}

	// Reachable if majority of probes succeeded
	if successes > cfg.Count/2 {
		avg := totalLatency
		if successes > 0 {
			avg = totalLatency / int64(successes)
		}
		return NetworkWitnessResult{Reachable: true, Latency: avg}
	}
	errMsg := "unreachable"
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	return NetworkWitnessResult{Reachable: false, Error: errMsg}
}

// probeHTTP does a GET to the target URL and considers any response a success.
// The target is a neutral vantage point; we only care if it responds at all.
func probeHTTP(target string, timeout time.Duration) error {
	url := target
	if len(url) < 4 || (url[:4] != "http") {
		url = "http://" + url
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{
		Timeout: timeout,
		// Don't follow redirects - any response means the target is reachable
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil // any HTTP response (even 404/503) means network is up
}

// probeICMP uses a TCP dial to the target on port 80 as a connectivity check.
// True ICMP requires raw sockets (root); TCP dial on a well-known port is a
// reliable network reachability test without elevated privileges.
// For a VPS target, port 80 (or 443) will almost always be open or filtered;
// either way, the TCP SYN reaching the host proves network connectivity.
func probeICMP(target string, timeout time.Duration) error {
	host := target
	// If target is a plain IP or hostname without port, try TCP:80
	if _, _, err := net.SplitHostPort(target); err != nil {
		host = net.JoinHostPort(target, "80")
	}
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		// Connection refused means the port is closed but the host is reachable
		if isConnectionRefused(err) {
			return nil
		}
		return fmt.Errorf("TCP probe to %s: %w", host, err)
	}
	conn.Close()
	return nil
}

func isConnectionRefused(err error) bool {
	if err == nil {
		return false
	}
	// "connection refused" means the host responded (it's reachable), just no listener
	msg := err.Error()
	return strings.Contains(msg, "connection refused") || strings.Contains(msg, "refused")
}

// GetNetworkWitnessConfig reads the witness config from the database.
func GetNetworkWitnessConfig(db *sql.DB) (NetworkWitnessConfig, error) {
	var cfg NetworkWitnessConfig
	err := db.QueryRow(`
		SELECT enable, target, method, timeout_ms, count, description
		FROM ha_network_witness WHERE id = 1`).
		Scan(&cfg.Enable, &cfg.Target, &cfg.Method,
			&cfg.TimeoutMs, &cfg.Count, &cfg.Description)
	if err == sql.ErrNoRows {
		return NetworkWitnessConfig{Method: "icmp", TimeoutMs: 2000, Count: 3}, nil
	}
	return cfg, err
}

// SaveNetworkWitnessConfig persists the witness config.
func SaveNetworkWitnessConfig(db *sql.DB, cfg NetworkWitnessConfig) error {
	if cfg.Target == "" && cfg.Enable {
		return fmt.Errorf("target is required when witness is enabled")
	}
	if cfg.Method != "icmp" && cfg.Method != "http" && cfg.Method != "https" {
		cfg.Method = "icmp"
	}
	if cfg.TimeoutMs < 100 {
		cfg.TimeoutMs = 2000
	}
	if cfg.Count < 1 || cfg.Count > 10 {
		cfg.Count = 3
	}
	_, err := db.Exec(`
		INSERT INTO ha_network_witness
			(id, enable, target, method, timeout_ms, count, description)
		VALUES (1, $1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			enable=$1, target=$2, method=$3,
			timeout_ms=$4, count=$5, description=$6`,
		cfg.Enable, cfg.Target, cfg.Method,
		cfg.TimeoutMs, cfg.Count, cfg.Description)
	return err
}
