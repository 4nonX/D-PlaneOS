package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// SystemStatusHandler handles system status, setup, and profile endpoints
type SystemStatusHandler struct {
	db        *sql.DB
	startTime time.Time
	version   string
}

func NewSystemStatusHandler(db *sql.DB) *SystemStatusHandler {
	return &SystemStatusHandler{
		db:        db,
		startTime: time.Now(),
		version:   "2.0.0",
	}
}

// ─── GET /api/system/status ────────────────────────────────
// Used by first-run-detection.js to check if system is set up

func (h *SystemStatusHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if setup has been completed
	var setupDone int
	h.db.QueryRow(`SELECT COUNT(*) FROM system_config WHERE key = 'setup_complete' AND value = '1'`).Scan(&setupDone)

	// Check if any users exist (beyond default)
	var userCount int
	h.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount)

	// Check if any pools exist
	poolOutput, _ := exec.Command("zpool", "list", "-H", "-o", "name").Output()
	pools := strings.Split(strings.TrimSpace(string(poolOutput)), "\n")
	poolCount := 0
	for _, p := range pools {
		if p != "" {
			poolCount++
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"version":        h.version,
		"setup_complete": setupDone > 0,
		"has_users":      userCount > 0,
		"has_pools":      poolCount > 0,
		"first_run":      setupDone == 0 && userCount <= 1,
		"uptime_seconds": int(time.Since(h.startTime).Seconds()),
	})
}

// ─── POST /api/system/setup-complete ───────────────────────

func (h *SystemStatusHandler) HandleSetupComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Ensure system_config table exists
	h.db.Exec(`CREATE TABLE IF NOT EXISTS system_config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	var body struct {
		Hostname string `json:"hostname"`
		Timezone string `json:"timezone"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	// Mark setup as complete
	h.db.Exec(`INSERT OR REPLACE INTO system_config (key, value) VALUES ('setup_complete', '1')`)

	// Save hostname if provided
	if body.Hostname != "" {
		h.db.Exec(`INSERT OR REPLACE INTO system_config (key, value) VALUES ('hostname', ?)`, body.Hostname)
		// Actually set the hostname
		exec.Command("hostnamectl", "set-hostname", body.Hostname).Run()
	}

	// Save timezone if provided
	if body.Timezone != "" {
		h.db.Exec(`INSERT OR REPLACE INTO system_config (key, value) VALUES ('timezone', ?)`, body.Timezone)
		exec.Command("timedatectl", "set-timezone", body.Timezone).Run()
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Setup completed",
	})
}

// ─── GET /api/system/profile ───────────────────────────────

func (h *SystemStatusHandler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hostname, _ := os.Hostname()

	// Get stored config values
	var storedHostname, timezone, description string
	h.db.QueryRow(`SELECT COALESCE(value,'') FROM system_config WHERE key = 'hostname'`).Scan(&storedHostname)
	h.db.QueryRow(`SELECT COALESCE(value,'') FROM system_config WHERE key = 'timezone'`).Scan(&timezone)
	h.db.QueryRow(`SELECT COALESCE(value,'') FROM system_config WHERE key = 'description'`).Scan(&description)

	if storedHostname != "" {
		hostname = storedHostname
	}
	if timezone == "" {
		tzBytes, _ := exec.Command("timedatectl", "show", "--property=Timezone", "--value").Output()
		timezone = strings.TrimSpace(string(tzBytes))
	}

	// Get kernel version
	kernel, _ := exec.Command("uname", "-r").Output()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"profile": map[string]interface{}{
			"hostname":    hostname,
			"timezone":    timezone,
			"description": description,
			"version":     h.version,
			"kernel":      strings.TrimSpace(string(kernel)),
			"arch":        runtime.GOARCH,
			"os":          runtime.GOOS,
			"uptime":      int(time.Since(h.startTime).Seconds()),
		},
	})
}

// ─── GET /api/system/preflight ─────────────────────────────

func (h *SystemStatusHandler) HandlePreflight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type check struct {
		Name    string `json:"name"`
		Status  string `json:"status"` // pass, warn, fail
		Message string `json:"message"`
	}

	var checks []check

	// 1. ZFS loaded
	_, err := exec.Command("which", "zpool").Output()
	if err != nil {
		checks = append(checks, check{"ZFS", "fail", "ZFS tools not installed"})
	} else {
		checks = append(checks, check{"ZFS", "pass", "ZFS available"})
	}

	// 2. Docker available
	_, err = exec.Command("which", "docker").Output()
	if err != nil {
		checks = append(checks, check{"Docker", "warn", "Docker not installed (containers unavailable)"})
	} else {
		checks = append(checks, check{"Docker", "pass", "Docker available"})
	}

	// 3. Samba available
	_, err = exec.Command("which", "smbd").Output()
	if err != nil {
		checks = append(checks, check{"Samba", "warn", "Samba not installed (SMB sharing unavailable)"})
	} else {
		checks = append(checks, check{"Samba", "pass", "Samba available"})
	}

	// 4. NFS available
	_, err = exec.Command("which", "exportfs").Output()
	if err != nil {
		checks = append(checks, check{"NFS", "warn", "NFS not installed (NFS sharing unavailable)"})
	} else {
		checks = append(checks, check{"NFS", "pass", "NFS available"})
	}

	// 5. Disk space on /
	dfOutput, _ := exec.Command("df", "-h", "/").Output()
	lines := strings.Split(string(dfOutput), "\n")
	diskMsg := "Unable to check"
	if len(lines) > 1 {
		diskMsg = strings.TrimSpace(lines[1])
	}
	checks = append(checks, check{"Root Disk", "pass", diskMsg})

	// 6. Memory
	memOutput, _ := exec.Command("free", "-h").Output()
	memLines := strings.Split(string(memOutput), "\n")
	memMsg := "Unable to check"
	if len(memLines) > 1 {
		memMsg = strings.TrimSpace(memLines[1])
	}
	checks = append(checks, check{"Memory", "pass", memMsg})

	// Overall status
	overallStatus := "pass"
	for _, c := range checks {
		if c.Status == "fail" {
			overallStatus = "fail"
			break
		}
		if c.Status == "warn" {
			overallStatus = "warn"
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"status":  overallStatus,
		"checks":  checks,
	})
}

// ─── GET/POST /api/system/settings (extends existing) ──────

func (h *SystemStatusHandler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	h.db.Exec(`CREATE TABLE IF NOT EXISTS system_config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	switch r.Method {
	case http.MethodGet:
		rows, err := h.db.Query(`SELECT key, value FROM system_config ORDER BY key`)
		if err != nil {
			respondErrorSimple(w, "Failed to read settings", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		settings := map[string]string{}
		for rows.Next() {
			var k, v string
			rows.Scan(&k, &v)
			settings[k] = v
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success":  true,
			"settings": settings,
		})

	case http.MethodPost:
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
			return
		}

		for k, v := range body {
			h.db.Exec(`INSERT OR REPLACE INTO system_config (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, k, v)
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("%d settings saved", len(body)),
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
