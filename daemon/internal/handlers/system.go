package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"dplaned/internal/audit"
	"dplaned/internal/security"
)

type SystemHandler struct{}

func NewSystemHandler() *SystemHandler {
	return &SystemHandler{}
}

func (h *SystemHandler) GetUPSStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := r.Header.Get("X-User")
	sessionID := r.Header.Get("X-Session-ID")

	if !security.IsValidSessionToken(sessionID) {
		audit.LogSecurityEvent("Invalid session token", user, r.RemoteAddr)
		respondErrorSimple(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if upsc exists
	start := time.Now()
	_, err := exec.LookPath("upsc")
	if err != nil {
		duration := time.Since(start)
		audit.LogCommand(audit.LevelInfo, user, "upsc_check", nil, false, duration, err)
		respondOK(w, CommandResponse{
			Success:  false,
			Error:    "NUT not installed",
			Duration: duration.Milliseconds(),
		})
		return
	}

	// Get UPS list
	output, err := executeCommand("/usr/bin/upsc", []string{"-l"})
	if err != nil || strings.TrimSpace(output) == "" {
		duration := time.Since(start)
		audit.LogCommand(audit.LevelInfo, user, "upsc_list", nil, false, duration, err)
		respondOK(w, CommandResponse{
			Success:  false,
			Error:    "No UPS found",
			Duration: duration.Milliseconds(),
		})
		return
	}

	// Get first UPS
	upsName := strings.TrimSpace(strings.Split(output, "\n")[0])

	// Get UPS data
	output, err = executeCommand("/usr/bin/upsc", []string{upsName})
	duration := time.Since(start)

	audit.LogCommand(audit.LevelInfo, user, "upsc_query", []string{upsName}, err == nil, duration, err)

	if err != nil {
		respondOK(w, CommandResponse{
			Success:  false,
			Error:    "Failed to read UPS",
			Duration: duration.Milliseconds(),
		})
		return
	}

	upsData := parseUPSData(output)

	respondOK(w, CommandResponse{
		Success: true,
		Data: map[string]interface{}{
			"battery_charge":  getUPSValue(upsData, "battery.charge", "N/A") + "%",
			"battery_runtime": getUPSValue(upsData, "battery.runtime", "N/A") + " sec",
			"status":          getUPSValue(upsData, "ups.status", "Unknown"),
			"model":           getUPSValue(upsData, "ups.model", "Unknown"),
			"manufacturer":    getUPSValue(upsData, "ups.mfr", "Unknown"),
			"serial":          getUPSValue(upsData, "ups.serial", "Unknown"),
			"load":            getUPSValue(upsData, "ups.load", "0"),
			"input_voltage":   getUPSValue(upsData, "input.voltage", "0"),
			"output_voltage":  getUPSValue(upsData, "output.voltage", "0"),
		},
		Duration: duration.Milliseconds(),
	})
}

func (h *SystemHandler) GetNetworkInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := r.Header.Get("X-User")
	sessionID := r.Header.Get("X-Session-ID")

	if !security.IsValidSessionToken(sessionID) {
		audit.LogSecurityEvent("Invalid session token", user, r.RemoteAddr)
		respondErrorSimple(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	start := time.Now()
	output, err := executeCommand("/usr/sbin/ip", []string{"-j", "addr", "show"})
	duration := time.Since(start)

	audit.LogCommand(audit.LevelInfo, user, "ip_addr", nil, err == nil, duration, err)

	if err != nil {
		respondOK(w, CommandResponse{
			Success:  false,
			Error:    err.Error(),
			Duration: duration.Milliseconds(),
		})
		return
	}

	var interfaces []interface{}
	if err := json.Unmarshal([]byte(output), &interfaces); err != nil {
		respondOK(w, CommandResponse{
			Success:  false,
			Error:    "Failed to parse network data",
			Duration: duration.Milliseconds(),
		})
		return
	}

	respondOK(w, CommandResponse{
		Success:  true,
		Data:     interfaces,
		Duration: duration.Milliseconds(),
	})
}

func (h *SystemHandler) GetSystemLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := r.Header.Get("X-User")
	sessionID := r.Header.Get("X-Session-ID")

	if !security.IsValidSessionToken(sessionID) {
		audit.LogSecurityEvent("Invalid session token", user, r.RemoteAddr)
		respondErrorSimple(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get line limit from query params (default 100)
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "100"
	}

	start := time.Now()
	output, err := executeCommand("/usr/bin/journalctl", []string{"-n", limit, "--no-pager", "-o", "json"})
	duration := time.Since(start)

	audit.LogCommand(audit.LevelInfo, user, "journalctl", []string{limit}, err == nil, duration, err)

	if err != nil {
		respondOK(w, CommandResponse{
			Success:  false,
			Error:    err.Error(),
			Duration: duration.Milliseconds(),
		})
		return
	}

	logs := parseJournalLogs(output)

	respondOK(w, CommandResponse{
		Success:  true,
		Data:     logs,
		Duration: duration.Milliseconds(),
	})
}

// Helper functions

func parseUPSData(output string) map[string]string {
	data := make(map[string]string)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			data[key] = value
		}
	}

	return data
}

func getUPSValue(data map[string]string, key, defaultValue string) string {
	if val, ok := data[key]; ok {
		return val
	}
	return defaultValue
}

func parseJournalLogs(output string) []map[string]interface{} {
	var logs []map[string]interface{}
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
			continue
		}

		// Extract relevant fields
		log := map[string]interface{}{
			"time":    fmt.Sprintf("%v", logEntry["__REALTIME_TIMESTAMP"]),
			"message": fmt.Sprintf("%v", logEntry["MESSAGE"]),
			"unit":    fmt.Sprintf("%v", logEntry["_SYSTEMD_UNIT"]),
		}

		// Determine level from priority
		if priority, ok := logEntry["PRIORITY"].(float64); ok {
			level := "info"
			if priority <= 3 {
				level = "error"
			} else if priority == 4 {
				level = "warning"
			}
			log["level"] = level
		}

		logs = append(logs, log)
	}

	return logs
}
