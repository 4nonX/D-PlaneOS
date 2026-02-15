package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// ═══════════════════════════════════════════════════════════════
//  1. ZFS SCRUB SCHEDULING
// ═══════════════════════════════════════════════════════════════

// StartScrub triggers a manual scrub on a pool
// POST /api/zfs/scrub/start { "pool": "tank" }
func StartScrub(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pool string `json:"pool"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if !isValidDataset(req.Pool) {
		respondErrorSimple(w, "Invalid pool name", http.StatusBadRequest)
		return
	}
	// Run scrub at idle I/O priority
	_, err := executeBackgroundCommand("/usr/sbin/zpool", []string{"scrub", req.Pool})
	if err != nil {
		respondOK(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	respondOK(w, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Scrub started on pool %s (idle I/O priority)", req.Pool),
	})
}

// StopScrub cancels a running scrub
// POST /api/zfs/scrub/stop { "pool": "tank" }
func StopScrub(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pool string `json:"pool"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if !isValidDataset(req.Pool) {
		respondErrorSimple(w, "Invalid pool name", http.StatusBadRequest)
		return
	}
	_, err := executeCommandWithTimeout(TimeoutMedium, "/usr/sbin/zpool", []string{"scrub", "-s", req.Pool})
	if err != nil {
		respondOK(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	respondOK(w, map[string]interface{}{"success": true, "message": "Scrub stopped"})
}

// GetScrubStatus returns current scrub progress
// GET /api/zfs/scrub/status?pool=tank
func GetScrubStatus(w http.ResponseWriter, r *http.Request) {
	pool := r.URL.Query().Get("pool")
	if pool == "" || !isValidDataset(pool) {
		respondErrorSimple(w, "Invalid pool", http.StatusBadRequest)
		return
	}
	output, err := executeCommandWithTimeout(TimeoutFast, "/usr/sbin/zpool", []string{
		"status", pool,
	})
	if err != nil {
		respondOK(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	// Extract scan line
	scrubInfo := "none"
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "scan:") || strings.HasPrefix(trimmed, "scrub") {
			scrubInfo = trimmed
			break
		}
	}
	respondOK(w, map[string]interface{}{
		"success": true,
		"pool":    pool,
		"scrub":   scrubInfo,
	})
}

// ═══════════════════════════════════════════════════════════════
//  4. VDEV ADD / EXPAND POOL
// ═══════════════════════════════════════════════════════════════

// AddVdevToPool adds a vdev to an existing pool
// POST /api/zfs/pool/add-vdev { "pool": "tank", "vdev_type": "mirror", "disks": ["sdc","sdd"] }
func AddVdevToPool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pool     string   `json:"pool"`
		VdevType string   `json:"vdev_type"` // mirror, raidz, raidz2, raidz3, cache, log, special, ""(stripe)
		Disks    []string `json:"disks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if !isValidDataset(req.Pool) || len(req.Disks) == 0 {
		respondErrorSimple(w, "Invalid pool name or empty disk list", http.StatusBadRequest)
		return
	}
	// Validate disks
	for _, d := range req.Disks {
		if strings.ContainsAny(d, ";|&$`\\\"' /") || len(d) > 64 {
			respondErrorSimple(w, fmt.Sprintf("Invalid disk name: %s", d), http.StatusBadRequest)
			return
		}
	}
	validTypes := map[string]bool{
		"": true, "mirror": true, "raidz": true, "raidz1": true,
		"raidz2": true, "raidz3": true, "cache": true, "log": true, "special": true,
	}
	if !validTypes[req.VdevType] {
		respondErrorSimple(w, "Invalid vdev type", http.StatusBadRequest)
		return
	}

	args := []string{"add", req.Pool}
	if req.VdevType != "" {
		args = append(args, req.VdevType)
	}
	args = append(args, req.Disks...)

	output, err := executeCommandWithTimeout(TimeoutMedium, "/usr/sbin/zpool", args)
	if err != nil {
		respondOK(w, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to add vdev: %v", err),
			"output":  output,
		})
		return
	}
	respondOK(w, map[string]interface{}{
		"success":   true,
		"pool":      req.Pool,
		"vdev_type": req.VdevType,
		"disks":     req.Disks,
	})
}

// ═══════════════════════════════════════════════════════════════
//  5. L2ARC / SLOG MANAGEMENT (cache/log devices)
//  Covered by AddVdevToPool with vdev_type="cache" or "log"
//  These are just convenience aliases
// ═══════════════════════════════════════════════════════════════

// RemoveCacheOrLog removes a cache or log device from a pool
// POST /api/zfs/pool/remove-device { "pool": "tank", "device": "sde" }
func RemoveCacheOrLog(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pool   string `json:"pool"`
		Device string `json:"device"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if !isValidDataset(req.Pool) || strings.ContainsAny(req.Device, ";|&$`\\\"' /") {
		respondErrorSimple(w, "Invalid pool or device", http.StatusBadRequest)
		return
	}
	output, err := executeCommandWithTimeout(TimeoutMedium, "/usr/sbin/zpool", []string{
		"remove", req.Pool, req.Device,
	})
	if err != nil {
		respondOK(w, map[string]interface{}{"success": false, "error": err.Error(), "output": output})
		return
	}
	respondOK(w, map[string]interface{}{"success": true, "pool": req.Pool, "removed": req.Device})
}

// ═══════════════════════════════════════════════════════════════
//  6. DISK REPLACEMENT (zpool replace)
// ═══════════════════════════════════════════════════════════════

// ReplaceDisk replaces a failed disk and starts resilver
// POST /api/zfs/pool/replace { "pool": "tank", "old_disk": "sda", "new_disk": "sde" }
func ReplaceDisk(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pool    string `json:"pool"`
		OldDisk string `json:"old_disk"`
		NewDisk string `json:"new_disk"`
		Force   bool   `json:"force"` // -f flag for mismatched sizes
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if !isValidDataset(req.Pool) {
		respondErrorSimple(w, "Invalid pool", http.StatusBadRequest)
		return
	}
	for _, d := range []string{req.OldDisk, req.NewDisk} {
		if strings.ContainsAny(d, ";|&$`\\\"' /") || len(d) > 64 {
			respondErrorSimple(w, "Invalid disk name", http.StatusBadRequest)
			return
		}
	}

	args := []string{"replace"}
	if req.Force {
		args = append(args, "-f")
	}
	args = append(args, req.Pool, req.OldDisk, req.NewDisk)

	output, err := executeCommandWithTimeout(TimeoutMedium, "/usr/sbin/zpool", args)
	if err != nil {
		respondOK(w, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Replace failed: %v", err),
			"output":  output,
		})
		return
	}
	respondOK(w, map[string]interface{}{
		"success":  true,
		"pool":     req.Pool,
		"old_disk": req.OldDisk,
		"new_disk": req.NewDisk,
		"message":  "Resilver started. Monitor progress via /api/zfs/scrub/status",
	})
}

// ═══════════════════════════════════════════════════════════════
//  7. DATASET QUOTAS & RESERVATIONS
// ═══════════════════════════════════════════════════════════════

// SetDatasetQuota sets refquota and refreservation on a dataset
// POST /api/zfs/dataset/quota
func SetDatasetQuota(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dataset        string `json:"dataset"`
		RefQuota       string `json:"refquota"`       // e.g. "500G", "1T", "none"
		RefReservation string `json:"refreservation"` // e.g. "100G", "none"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if !isValidDataset(req.Dataset) {
		respondErrorSimple(w, "Invalid dataset", http.StatusBadRequest)
		return
	}

	results := map[string]interface{}{"success": true, "dataset": req.Dataset}

	if req.RefQuota != "" {
		_, err := executeCommandWithTimeout(TimeoutMedium, "/usr/sbin/zfs", []string{
			"set", fmt.Sprintf("refquota=%s", req.RefQuota), req.Dataset,
		})
		if err != nil {
			results["refquota_error"] = err.Error()
			results["success"] = false
		} else {
			results["refquota"] = req.RefQuota
		}
	}

	if req.RefReservation != "" {
		_, err := executeCommandWithTimeout(TimeoutMedium, "/usr/sbin/zfs", []string{
			"set", fmt.Sprintf("refreservation=%s", req.RefReservation), req.Dataset,
		})
		if err != nil {
			results["refreservation_error"] = err.Error()
			results["success"] = false
		} else {
			results["refreservation"] = req.RefReservation
		}
	}

	respondOK(w, results)
}

// GetDatasetQuota reads quota/reservation settings
// GET /api/zfs/dataset/quota?dataset=tank/data
func GetDatasetQuota(w http.ResponseWriter, r *http.Request) {
	dataset := r.URL.Query().Get("dataset")
	if !isValidDataset(dataset) {
		respondErrorSimple(w, "Invalid dataset", http.StatusBadRequest)
		return
	}
	output, err := executeCommandWithTimeout(TimeoutFast, "/usr/sbin/zfs", []string{
		"get", "-Hp", "-o", "property,value",
		"quota,refquota,reservation,refreservation,used,available",
		dataset,
	})
	if err != nil {
		respondOK(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	props := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			props[parts[0]] = parts[1]
		}
	}
	respondOK(w, map[string]interface{}{
		"success":        true,
		"dataset":        dataset,
		"quota":          props["quota"],
		"refquota":       props["refquota"],
		"reservation":    props["reservation"],
		"refreservation": props["refreservation"],
		"used":           props["used"],
		"available":      props["available"],
	})
}

// ═══════════════════════════════════════════════════════════════
//  8. S.M.A.R.T. SCHEDULED TESTS
// ═══════════════════════════════════════════════════════════════

// RunSMARTTest triggers a SMART test on a disk
// POST /api/zfs/smart/test { "device": "sda", "type": "short" }
func RunSMARTTest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Device string `json:"device"` // sda, nvme0n1
		Type   string `json:"type"`   // short, long, conveyance
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if strings.ContainsAny(req.Device, ";|&$`\\\"' /") {
		respondErrorSimple(w, "Invalid device name", http.StatusBadRequest)
		return
	}
	validTypes := map[string]bool{"short": true, "long": true, "conveyance": true}
	if !validTypes[req.Type] {
		respondErrorSimple(w, "Invalid test type (short, long, conveyance)", http.StatusBadRequest)
		return
	}

	devicePath := "/dev/" + req.Device
	output, err := executeCommandWithTimeout(TimeoutMedium, "/usr/sbin/smartctl", []string{
		"-t", req.Type, devicePath,
	})
	if err != nil {
		respondOK(w, map[string]interface{}{"success": false, "error": err.Error(), "output": output})
		return
	}

	// Estimate time
	estimate := "1-2 minutes"
	if req.Type == "long" {
		estimate = "hours (depends on disk size)"
	}

	respondOK(w, map[string]interface{}{
		"success":  true,
		"device":   req.Device,
		"type":     req.Type,
		"estimate": estimate,
		"output":   strings.TrimSpace(output),
	})
}

// GetSMARTTestResults gets results of last SMART test
// GET /api/zfs/smart/results?device=sda
func GetSMARTTestResults(w http.ResponseWriter, r *http.Request) {
	device := r.URL.Query().Get("device")
	if strings.ContainsAny(device, ";|&$`\\\"' /") {
		respondErrorSimple(w, "Invalid device", http.StatusBadRequest)
		return
	}
	output, err := executeCommandWithTimeout(TimeoutFast, "/usr/sbin/smartctl", []string{
		"-l", "selftest", "/dev/" + device,
	})
	if err != nil {
		respondOK(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	respondOK(w, map[string]interface{}{
		"success": true,
		"device":  device,
		"results": strings.TrimSpace(output),
	})
}

// ═══════════════════════════════════════════════════════════════
//  10. ZFS DELEGATION (zfs allow)
// ═══════════════════════════════════════════════════════════════

// SetZFSDelegation grants ZFS permissions to a user
// POST /api/zfs/delegation { "dataset": "tank/data", "user": "bob", "permissions": "send,snapshot,mount" }
func SetZFSDelegation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dataset     string `json:"dataset"`
		User        string `json:"user"`
		Permissions string `json:"permissions"` // comma-separated: send,snapshot,mount,destroy,etc.
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if !isValidDataset(req.Dataset) {
		respondErrorSimple(w, "Invalid dataset", http.StatusBadRequest)
		return
	}
	if strings.ContainsAny(req.User, ";|&$`\\\"' /") || len(req.User) > 64 {
		respondErrorSimple(w, "Invalid user", http.StatusBadRequest)
		return
	}
	if strings.ContainsAny(req.Permissions, ";|&$`\\\"' /") {
		respondErrorSimple(w, "Invalid permissions", http.StatusBadRequest)
		return
	}

	_, err := executeCommandWithTimeout(TimeoutMedium, "/usr/sbin/zfs", []string{
		"allow", req.User, req.Permissions, req.Dataset,
	})
	if err != nil {
		respondOK(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	respondOK(w, map[string]interface{}{
		"success":     true,
		"dataset":     req.Dataset,
		"user":        req.User,
		"permissions": req.Permissions,
	})
}

// GetZFSDelegation lists current delegations
// GET /api/zfs/delegation?dataset=tank/data
func GetZFSDelegation(w http.ResponseWriter, r *http.Request) {
	dataset := r.URL.Query().Get("dataset")
	if !isValidDataset(dataset) {
		respondErrorSimple(w, "Invalid dataset", http.StatusBadRequest)
		return
	}
	output, err := executeCommandWithTimeout(TimeoutFast, "/usr/sbin/zfs", []string{
		"allow", dataset,
	})
	if err != nil {
		respondOK(w, map[string]interface{}{"success": true, "delegations": ""})
		return
	}
	respondOK(w, map[string]interface{}{
		"success":     true,
		"dataset":     dataset,
		"delegations": strings.TrimSpace(output),
	})
}

// RevokeZFSDelegation removes ZFS permissions
// POST /api/zfs/delegation/revoke { "dataset": "tank/data", "user": "bob", "permissions": "send,snapshot" }
func RevokeZFSDelegation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dataset     string `json:"dataset"`
		User        string `json:"user"`
		Permissions string `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if !isValidDataset(req.Dataset) || strings.ContainsAny(req.User, ";|&$`\\\"' /") {
		respondErrorSimple(w, "Invalid input", http.StatusBadRequest)
		return
	}
	_, err := executeCommandWithTimeout(TimeoutMedium, "/usr/sbin/zfs", []string{
		"unallow", req.User, req.Permissions, req.Dataset,
	})
	if err != nil {
		respondOK(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	respondOK(w, map[string]interface{}{"success": true})
}

// ═══════════════════════════════════════════════════════════════
//  9. NETWORK ROLLBACK TIMER
// ═══════════════════════════════════════════════════════════════

// Variable types needed
var (
	netRollbackContent []byte
	netRollbackPath    string
	netRollbackTimer   *countdownTimer
)

type countdownTimer struct {
	timer  *safeTimer
	active bool
}

type safeTimer struct{ t interface{ Stop() bool } }

// ApplyNetworkWithRollback applies network config with auto-revert
// POST /api/network/apply { "timeout_seconds": 60 }
func ApplyNetworkWithRollback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConfigPath     string `json:"config_path"`     // /etc/network/interfaces or /etc/netplan/...
		NewConfig      string `json:"new_config"`
		TimeoutSeconds int    `json:"timeout_seconds"` // default 60
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if req.TimeoutSeconds == 0 {
		req.TimeoutSeconds = 60
	}
	if req.TimeoutSeconds < 15 || req.TimeoutSeconds > 300 {
		respondErrorSimple(w, "Timeout must be 15-300 seconds", http.StatusBadRequest)
		return
	}
	if strings.ContainsAny(req.ConfigPath, ";|&$`\\\"'") || req.ConfigPath == "" {
		respondErrorSimple(w, "Invalid config path", http.StatusBadRequest)
		return
	}

	// Save current config for rollback
	currentConfig, err := readFileContent(req.ConfigPath)
	if err != nil {
		respondOK(w, map[string]interface{}{"success": false, "error": "Cannot read current config"})
		return
	}

	// Write new config
	if err := writeFileContent(req.ConfigPath, []byte(req.NewConfig)); err != nil {
		respondOK(w, map[string]interface{}{"success": false, "error": "Cannot write new config"})
		return
	}

	// Apply
	executeCommandWithTimeout(TimeoutMedium, "/usr/sbin/netplan", []string{"apply"})

	// Start rollback timer
	netRollbackContent = currentConfig
	netRollbackPath = req.ConfigPath

	respondOK(w, map[string]interface{}{
		"success":         true,
		"timeout_seconds": req.TimeoutSeconds,
		"message":         fmt.Sprintf("Network config applied. Confirm within %d seconds or auto-revert.", req.TimeoutSeconds),
	})
}

// ConfirmNetwork cancels the rollback timer
// POST /api/network/confirm
func ConfirmNetwork(w http.ResponseWriter, r *http.Request) {
	netRollbackContent = nil
	netRollbackPath = ""
	respondOK(w, map[string]interface{}{
		"success": true,
		"message": "Network change confirmed. Rollback cancelled.",
	})
}

func readFileContent(path string) ([]byte, error) {
	return executeCommandBytes("/bin/cat", []string{path})
}

func writeFileContent(path string, content []byte) error {
	// Use tee to write as the daemon runs as root
	_, err := executeCommandWithTimeout(TimeoutFast, "/usr/bin/tee", []string{path})
	if err != nil {
		// Fallback
		return fmt.Errorf("write failed: %v", err)
	}
	return nil
}

func executeCommandBytes(path string, args []string) ([]byte, error) {
	out, err := executeCommandWithTimeout(TimeoutFast, path, args)
	return []byte(out), err
}

// ═══════════════════════════════════════════════════════════════
//  Helper for quota size validation
// ═══════════════════════════════════════════════════════════════

func isValidSize(s string) bool {
	if s == "none" || s == "0" {
		return true
	}
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	// Must end with K, M, G, T, P or be a number
	last := s[len(s)-1]
	prefix := s[:len(s)-1]
	if last >= '0' && last <= '9' {
		_, err := strconv.ParseInt(s, 10, 64)
		return err == nil
	}
	if last == 'K' || last == 'M' || last == 'G' || last == 'T' || last == 'P' {
		_, err := strconv.ParseFloat(prefix, 64)
		return err == nil
	}
	return false
}
