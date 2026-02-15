package handlers

import (
	"database/sql"
	"dplaned/internal/cmdutil"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

type SystemStatusHandler struct {
	db        *sql.DB
	startTime time.Time
	version   string
}

func NewSystemStatusHandler(db *sql.DB) *SystemStatusHandler {
	return &SystemStatusHandler{db: db, startTime: time.Now(), version: "2.1.0"}
}

func (h *SystemStatusHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var setupDone int
	h.db.QueryRow(`SELECT COUNT(*) FROM system_config WHERE key = 'setup_complete' AND value = '1'`).Scan(&setupDone)
	var userCount int
	h.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount)

	poolOutput, err := cmdutil.RunFast("zpool", "list", "-H", "-o", "name")
	if err != nil {
		log.Printf("WARN: zpool list: %v", err)
	}
	pools := strings.Split(strings.TrimSpace(string(poolOutput)), "\n")
	poolCount := 0
	for _, p := range pools {
		if p != "" { poolCount++ }
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true, "version": h.version, "setup_complete": setupDone > 0,
		"has_users": userCount > 0, "has_pools": poolCount > 0,
		"first_run": setupDone == 0 && userCount <= 1,
		"uptime_seconds": int(time.Since(h.startTime).Seconds()),
	})
}

func (h *SystemStatusHandler) HandleSetupComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.db.Exec(`CREATE TABLE IF NOT EXISTS system_config (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
	var body struct { Hostname string `json:"hostname"`; Timezone string `json:"timezone"` }
	json.NewDecoder(r.Body).Decode(&body)
	h.db.Exec(`INSERT OR REPLACE INTO system_config (key, value) VALUES ('setup_complete', '1')`)
	if body.Hostname != "" {
		h.db.Exec(`INSERT OR REPLACE INTO system_config (key, value) VALUES ('hostname', ?)`, body.Hostname)
		if _, err := cmdutil.RunFast("hostnamectl", "set-hostname", body.Hostname); err != nil {
			log.Printf("WARN: hostnamectl: %v", err)
		}
	}
	if body.Timezone != "" {
		h.db.Exec(`INSERT OR REPLACE INTO system_config (key, value) VALUES ('timezone', ?)`, body.Timezone)
		if _, err := cmdutil.RunFast("timedatectl", "set-timezone", body.Timezone); err != nil {
			log.Printf("WARN: timedatectl: %v", err)
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "Setup completed"})
}

func (h *SystemStatusHandler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	hostname, err := os.Hostname()
	if err != nil { log.Printf("WARN: os.Hostname: %v", err); hostname = "unknown" }
	var storedHostname, timezone, description string
	h.db.QueryRow(`SELECT COALESCE(value,'') FROM system_config WHERE key = 'hostname'`).Scan(&storedHostname)
	h.db.QueryRow(`SELECT COALESCE(value,'') FROM system_config WHERE key = 'timezone'`).Scan(&timezone)
	h.db.QueryRow(`SELECT COALESCE(value,'') FROM system_config WHERE key = 'description'`).Scan(&description)
	if storedHostname != "" { hostname = storedHostname }
	if timezone == "" {
		if tzBytes, err := cmdutil.RunFast("timedatectl", "show", "--property=Timezone", "--value"); err != nil {
			log.Printf("WARN: timedatectl: %v", err)
		} else { timezone = strings.TrimSpace(string(tzBytes)) }
	}
	kernel, err := cmdutil.RunFast("uname", "-r")
	if err != nil { log.Printf("WARN: uname: %v", err) }
	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "profile": map[string]interface{}{
		"hostname": hostname, "timezone": timezone, "description": description,
		"version": h.version, "kernel": strings.TrimSpace(string(kernel)),
		"arch": runtime.GOARCH, "os": runtime.GOOS, "uptime": int(time.Since(h.startTime).Seconds()),
	}})
}

func (h *SystemStatusHandler) HandlePreflight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	type check struct { Name string `json:"name"`; Status string `json:"status"`; Message string `json:"message"` }
	var checks []check
	if _, err := cmdutil.RunFast("which", "zpool"); err != nil {
		checks = append(checks, check{"ZFS", "fail", "ZFS tools not installed"})
	} else { checks = append(checks, check{"ZFS", "pass", "ZFS available"}) }
	if _, err := cmdutil.RunFast("which", "docker"); err != nil {
		checks = append(checks, check{"Docker", "warn", "Docker not installed"})
	} else { checks = append(checks, check{"Docker", "pass", "Docker available"}) }
	if _, err := cmdutil.RunFast("which", "smbd"); err != nil {
		checks = append(checks, check{"Samba", "warn", "Samba not installed"})
	} else { checks = append(checks, check{"Samba", "pass", "Samba available"}) }
	if _, err := cmdutil.RunFast("which", "exportfs"); err != nil {
		checks = append(checks, check{"NFS", "warn", "NFS not installed"})
	} else { checks = append(checks, check{"NFS", "pass", "NFS available"}) }
	dfOut, err := cmdutil.RunFast("df", "-h", "/"); diskMsg := "Unable to check"
	if err != nil { log.Printf("WARN: df: %v", err) } else {
		lines := strings.Split(string(dfOut), "\n"); if len(lines) > 1 { diskMsg = strings.TrimSpace(lines[1]) }
	}
	checks = append(checks, check{"Root Disk", "pass", diskMsg})
	memOut, err := cmdutil.RunFast("free", "-h"); memMsg := "Unable to check"
	if err != nil { log.Printf("WARN: free: %v", err) } else {
		lines := strings.Split(string(memOut), "\n"); if len(lines) > 1 { memMsg = strings.TrimSpace(lines[1]) }
	}
	checks = append(checks, check{"Memory", "pass", memMsg})
	overall := "pass"
	for _, c := range checks { if c.Status == "fail" { overall = "fail"; break }; if c.Status == "warn" { overall = "warn" } }
	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "status": overall, "checks": checks})
}

func (h *SystemStatusHandler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	h.db.Exec(`CREATE TABLE IF NOT EXISTS system_config (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
	switch r.Method {
	case http.MethodGet:
		rows, err := h.db.Query(`SELECT key, value FROM system_config ORDER BY key`)
		if err != nil { respondErrorSimple(w, "Failed to read settings", http.StatusInternalServerError); return }
		defer rows.Close()
		settings := map[string]string{}
		for rows.Next() { var k, v string; rows.Scan(&k, &v); settings[k] = v }
		respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "settings": settings})
	case http.MethodPost:
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil { respondErrorSimple(w, "Invalid request", http.StatusBadRequest); return }
		for k, v := range body { h.db.Exec(`INSERT OR REPLACE INTO system_config (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, k, v) }
		respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": fmt.Sprintf("%d settings saved", len(body))})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
