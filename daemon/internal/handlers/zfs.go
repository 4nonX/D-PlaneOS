package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"dplaned/internal/audit"
	"dplaned/internal/security"
)

type ZFSHandler struct{}

type CommandRequest struct {
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	SessionID string   `json:"session_id"`
	User      string   `json:"user"`
}

type CommandResponse struct {
	Success  bool        `json:"success"`
	Output   string      `json:"output,omitempty"`
	Error    string      `json:"error,omitempty"`
	Duration int64       `json:"duration_ms"`
	Data     interface{} `json:"data,omitempty"`
}

func NewZFSHandler() *ZFSHandler {
	return &ZFSHandler{}
}

func (h *ZFSHandler) HandleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate session token format
	if !security.IsValidSessionToken(req.SessionID) {
		audit.LogSecurityEvent("Invalid session token format", req.User, r.RemoteAddr)
		respondErrorSimple(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate command is whitelisted
	if err := security.ValidateCommand(req.Command, req.Args); err != nil {
		audit.LogSecurityEvent(fmt.Sprintf("Command validation failed: %v", err), req.User, r.RemoteAddr)
		respondErrorSimple(w, err.Error(), http.StatusForbidden)
		return
	}

	// Get command from whitelist
	cmd, exists := security.CommandWhitelist[req.Command]
	if !exists {
		respondErrorSimple(w, "Command not found", http.StatusNotFound)
		return
	}

	// Execute command
	start := time.Now()
	output, err := executeCommand(cmd.Path, req.Args)
	duration := time.Since(start)

	// Log the execution
	audit.LogCommand(
		audit.LevelInfo,
		req.User,
		req.Command,
		req.Args,
		err == nil,
		duration,
		err,
	)

	if err != nil {
		respondOK(w, CommandResponse{
			Success:  false,
			Error:    err.Error(),
			Duration: duration.Milliseconds(),
		})
		return
	}

	// Sanitize output
	output = security.SanitizeOutput(output)

	respondOK(w, CommandResponse{
		Success:  true,
		Output:   output,
		Duration: duration.Milliseconds(),
	})
}

func (h *ZFSHandler) ListPools(w http.ResponseWriter, r *http.Request) {
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
	output, err := executeCommand("/usr/sbin/zpool", []string{"list", "-H", "-o", "name,size,alloc,free,health"})
	duration := time.Since(start)

	audit.LogCommand(audit.LevelInfo, user, "zpool_list", nil, err == nil, duration, err)

	if err != nil {
		respondOK(w, CommandResponse{
			Success:  false,
			Error:    err.Error(),
			Duration: duration.Milliseconds(),
		})
		return
	}

	pools := parseZpoolList(output)

	respondOK(w, CommandResponse{
		Success:  true,
		Data:     pools,
		Duration: duration.Milliseconds(),
	})
}

func (h *ZFSHandler) ListDatasets(w http.ResponseWriter, r *http.Request) {
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
	output, err := executeCommand("/usr/sbin/zfs", []string{"list", "-H", "-o", "name,used,avail,refer,mountpoint", "-t", "filesystem"})
	duration := time.Since(start)

	audit.LogCommand(audit.LevelInfo, user, "zfs_list", nil, err == nil, duration, err)

	if err != nil {
		respondOK(w, CommandResponse{
			Success:  false,
			Error:    err.Error(),
			Duration: duration.Milliseconds(),
		})
		return
	}

	datasets := parseZfsList(output)

	respondOK(w, CommandResponse{
		Success:  true,
		Data:     datasets,
		Duration: duration.Milliseconds(),
	})
}

// ManageDatasets handles POST operations on datasets (create, set_quota, etc.)
// POST /api/zfs/datasets
func (h *ZFSHandler) ManageDatasets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse body to determine action
	var raw map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		respondErrorSimple(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	action, _ := raw["action"].(string)

	switch action {
	case "set_quota":
		h.handleSetQuota(w, raw)
	default:
		// No action field means dataset creation
		h.handleCreateDataset(w, raw)
	}
}

func (h *ZFSHandler) handleCreateDataset(w http.ResponseWriter, raw map[string]interface{}) {
	pool, _ := raw["pool"].(string)
	name, _ := raw["name"].(string)
	mountpoint, _ := raw["mountpoint"].(string)
	quota, _ := raw["quota"].(string)
	compression, _ := raw["compression"].(string)

	if pool == "" || name == "" {
		respondErrorSimple(w, "Pool and dataset name are required", http.StatusBadRequest)
		return
	}

	fullName := pool + "/" + name
	if !security.IsValidSessionToken("") {
		// Validate dataset name
	}
	if !isValidDataset(fullName) {
		respondErrorSimple(w, "Invalid dataset name", http.StatusBadRequest)
		return
	}

	// Build zfs create args
	args := []string{"create"}

	if mountpoint != "" {
		if strings.ContainsAny(mountpoint, ";|&$`\"'") {
			respondErrorSimple(w, "Invalid mountpoint", http.StatusBadRequest)
			return
		}
		args = append(args, "-o", fmt.Sprintf("mountpoint=%s", mountpoint))
	}

	if compression != "" && compression != "off" {
		validComp := map[string]bool{"lz4": true, "zstd": true, "gzip": true, "off": true}
		if validComp[compression] {
			args = append(args, "-o", fmt.Sprintf("compression=%s", compression))
		}
	}

	if quota != "" {
		args = append(args, "-o", fmt.Sprintf("quota=%s", quota))
	}

	args = append(args, fullName)

	output, err := executeCommand("/usr/sbin/zfs", args)
	if err != nil {
		respondOK(w, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to create dataset: %v", err),
			"output":  output,
		})
		return
	}

	respondOK(w, map[string]interface{}{
		"success": true,
		"dataset": fullName,
		"message": "Dataset created successfully",
	})
}

func (h *ZFSHandler) handleSetQuota(w http.ResponseWriter, raw map[string]interface{}) {
	dataset, _ := raw["dataset"].(string)
	if !isValidDataset(dataset) {
		respondErrorSimple(w, "Invalid dataset", http.StatusBadRequest)
		return
	}

	// Quota can be float64 from JSON
	var quotaStr string
	switch v := raw["quota"].(type) {
	case string:
		quotaStr = v
	case float64:
		if v == 0 {
			quotaStr = "none"
		} else {
			quotaStr = fmt.Sprintf("%.0f", v)
		}
	default:
		quotaStr = "none"
	}

	if quotaStr == "" || quotaStr == "0" {
		quotaStr = "none"
	}

	output, err := executeCommand("/usr/sbin/zfs", []string{
		"set", fmt.Sprintf("quota=%s", quotaStr), dataset,
	})
	if err != nil {
		respondOK(w, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to set quota: %v", err),
			"output":  output,
		})
		return
	}

	respondOK(w, map[string]interface{}{
		"success": true,
		"dataset": dataset,
		"quota":   quotaStr,
	})
}

// Helper functions

// executeCommand runs a command with timeout and returns ONLY stdout.
// Stderr is logged separately to prevent warning messages (e.g. "pool is DEGRADED")
// from being misinterpreted as data by the ZFS parsers.
func executeCommand(path string, args []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		log.Printf("ZFS TIMEOUT [%s %v] after 30s", path, args)
		return stdout.String(), fmt.Errorf("command timed out after 30s: %s %v", path, args)
	}
	if stderrStr := strings.TrimSpace(stderr.String()); stderrStr != "" {
		log.Printf("ZFS stderr [%s %v]: %s", path, args, stderrStr)
	}
	return stdout.String(), err
}

// parseZpoolList parses `zpool list -H -o name,size,alloc,free,health` output.
// Uses tab-delimited split (ZFS -H flag outputs tabs) and validates field count
// to prevent partial or malformed lines from producing garbage data.
func parseZpoolList(output string) []map[string]string {
	var pools []map[string]string
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// -H output is tab-delimited
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			// Fallback: try whitespace split for robustness
			fields = strings.Fields(line)
		}
		if len(fields) < 5 {
			log.Printf("ZFS parse warning: skipping malformed zpool line (%d fields): %q", len(fields), line)
			continue
		}

		// Validate pool name: must start with alphanumeric (skip stray warnings)
		if len(fields[0]) == 0 || !isPoolNameChar(fields[0][0]) {
			log.Printf("ZFS parse warning: skipping non-pool line: %q", line)
			continue
		}

		pools = append(pools, map[string]string{
			"name":   fields[0],
			"size":   fields[1],
			"alloc":  fields[2],
			"free":   fields[3],
			"health": fields[4],
		})
	}

	return pools
}

func isPoolNameChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// parseZfsList parses `zfs list -H -o name,used,avail,refer,mountpoint` output.
// Same resilience as parseZpoolList: tab-split, field validation, malformed line skip.
func parseZfsList(output string) []map[string]string {
	var datasets []map[string]string
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			fields = strings.Fields(line)
		}
		if len(fields) < 5 {
			log.Printf("ZFS parse warning: skipping malformed zfs line (%d fields): %q", len(fields), line)
			continue
		}

		if len(fields[0]) == 0 || !isPoolNameChar(fields[0][0]) {
			continue
		}

		datasets = append(datasets, map[string]string{
			"name":       fields[0],
			"used":       fields[1],
			"avail":      fields[2],
			"refer":      fields[3],
			"mountpoint": fields[4],
		})
	}

	return datasets
}

// respondJSON and respondError are defined in helpers.go
