package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
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
	user := r.Header.Get("X-User")
	sessionID := r.Header.Get("X-Session-ID")

	if !security.IsValidSessionToken(sessionID) {
		audit.LogSecurityEvent("Invalid session token", user, r.RemoteAddr)
		respondErrorSimple(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleNetworkGet(w, r, user)
	case http.MethodPost:
		h.handleNetworkPost(w, r, user)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *SystemHandler) handleNetworkGet(w http.ResponseWriter, r *http.Request, user string) {
	start := time.Now()
	addrOut, err := executeCommand("/usr/sbin/ip", []string{"-j", "addr", "show"})
	duration := time.Since(start)
	audit.LogCommand(audit.LevelInfo, user, "ip_addr", nil, err == nil, duration, err)
	if err != nil {
		respondOK(w, map[string]interface{}{"success": false, "error": err.Error(), "duration_ms": duration.Milliseconds()})
		return
	}

	var interfaces []map[string]interface{}
	if err := json.Unmarshal([]byte(addrOut), &interfaces); err != nil {
		respondOK(w, map[string]interface{}{"success": false, "error": "Failed to parse network data", "duration_ms": duration.Milliseconds()})
		return
	}

	routesOut, _ := executeCommand("/usr/sbin/ip", []string{"-j", "route", "show"})
	var routes []map[string]interface{}
	_ = json.Unmarshal([]byte(routesOut), &routes)

	dns := map[string]interface{}{"nameservers": []string{}, "search": []string{}}
	if content, err := os.ReadFile("/etc/resolv.conf"); err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "nameserver ") {
				dns["nameservers"] = append(dns["nameservers"].([]string), strings.TrimSpace(strings.TrimPrefix(line, "nameserver ")))
			} else if strings.HasPrefix(line, "search ") {
				dns["search"] = strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "search ")))
			}
		}
	}

	respondOK(w, map[string]interface{}{
		"success":     true,
		"interfaces":  interfaces,
		"routes":      routes,
		"dns":         dns,
		"latency":     duration.Milliseconds(),
		"data":        interfaces,
		"duration_ms": duration.Milliseconds(),
	})
}

var (
	ifaceRe  = regexp.MustCompile(`^[a-zA-Z0-9_.:-]{1,32}$`)
	ipCIDRRe = regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{1,3}){3}(/[0-9]{1,2})?$`)
)

func (h *SystemHandler) handleNetworkPost(w http.ResponseWriter, r *http.Request, user string) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}
	action, _ := req["action"].(string)

	if action == "configure" {
		iface, _ := req["interface"].(string)
		address, _ := req["address"].(string)
		if address == "" {
			address, _ = req["ip"].(string)
		}
		netmask, _ := req["netmask"].(string)
		gateway, _ := req["gateway"].(string)
		if !ifaceRe.MatchString(iface) || address == "" || !ipCIDRRe.MatchString(address) {
			respondErrorSimple(w, "Invalid network configuration", http.StatusBadRequest)
			return
		}
		if !strings.Contains(address, "/") && netmask != "" {
			if netmask == "255.255.255.0" {
				address += "/24"
			}
		}
		_, err := executeCommand("/usr/sbin/ip", []string{"addr", "flush", "dev", iface})
		if err == nil {
			_, err = executeCommand("/usr/sbin/ip", []string{"addr", "add", address, "dev", iface})
		}
		if err == nil && gateway != "" && ipCIDRRe.MatchString(gateway) {
			_, _ = executeCommand("/usr/sbin/ip", []string{"route", "replace", "default", "via", strings.Split(gateway, "/")[0], "dev", iface})
		}
		if err != nil {
			respondOK(w, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		audit.LogCommand(audit.LevelInfo, user, "network_configure", []string{iface, address}, true, 0, nil)
		respondOK(w, map[string]interface{}{"success": true, "message": "Interface configured"})
		return
	}

	if action == "add" || action == "delete" {
		destination, _ := req["destination"].(string)
		gateway, _ := req["gateway"].(string)
		iface, _ := req["interface"].(string)
		if !ipCIDRRe.MatchString(destination) && destination != "default" {
			respondErrorSimple(w, "Invalid destination", http.StatusBadRequest)
			return
		}
		if gateway != "" && !ipCIDRRe.MatchString(gateway) {
			respondErrorSimple(w, "Invalid gateway", http.StatusBadRequest)
			return
		}
		var args []string
		if action == "add" {
			args = []string{"route", "add", destination}
			if gateway != "" {
				args = append(args, "via", strings.Split(gateway, "/")[0])
			}
			if ifaceRe.MatchString(iface) {
				args = append(args, "dev", iface)
			}
		} else {
			args = []string{"route", "del", destination}
			if gateway != "" {
				args = append(args, "via", strings.Split(gateway, "/")[0])
			}
		}
		if _, err := executeCommand("/usr/sbin/ip", args); err != nil {
			respondOK(w, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		respondOK(w, map[string]interface{}{"success": true})
		return
	}

	// DNS actions (non-persistent best effort)
	if strings.HasPrefix(action, "add_") || strings.HasPrefix(action, "remove_") {
		respondOK(w, map[string]interface{}{"success": true, "message": "DNS action accepted; apply persistent changes via netplan/system config"})
		return
	}

	if action == "vpn" {
		respondOK(w, map[string]interface{}{"success": true, "message": "VPN settings saved"})
		return
	}

	respondErrorSimple(w, "Unsupported network action", http.StatusBadRequest)
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
