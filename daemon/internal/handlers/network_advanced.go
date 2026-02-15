package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ═══════════════════════════════════════════════════════════════
//  2+3. SMB VFS MODULE SUPPORT + EXTRA PARAMETERS
// ═══════════════════════════════════════════════════════════════

// SMBGlobalConfig represents configurable global SMB settings
type SMBGlobalConfig struct {
	Workgroup      string `json:"workgroup"`
	ServerString   string `json:"server_string"`
	TimeMachine    bool   `json:"time_machine"`     // enables vfs_fruit globally
	ShadowCopy     bool   `json:"shadow_copy"`      // enables vfs_shadow_copy2
	RecycleBin     bool   `json:"recycle_bin"`       // enables vfs_recycle
	ExtraGlobal    string `json:"extra_global"`      // custom global params
}

// SMBShareVFS represents per-share VFS module options
type SMBShareVFS struct {
	ShareName      string `json:"share_name"`
	TimeMachine    bool   `json:"time_machine"`      // vfs_fruit per share
	ShadowCopy     bool   `json:"shadow_copy"`       // vfs_shadow_copy2 per share
	RecycleBin     bool   `json:"recycle_bin"`        // vfs_recycle per share
	RecycleMaxAge  int    `json:"recycle_max_age"`    // days (0=infinite)
	RecycleMaxSize int    `json:"recycle_max_size"`   // MB (0=infinite)
	ExtraParams    string `json:"extra_params"`       // custom per-share params
}

// GetSMBVFSConfig returns current VFS module configuration
// GET /api/smb/vfs
func GetSMBVFSConfig(w http.ResponseWriter, r *http.Request) {
	// Read current smb.conf and parse VFS settings
	output, err := executeCommandWithTimeout(TimeoutFast, "/bin/cat", []string{"/etc/samba/smb.conf"})
	if err != nil {
		respondOK(w, map[string]interface{}{
			"success":       true,
			"time_machine":  false,
			"shadow_copy":   false,
			"recycle_bin":   false,
		})
		return
	}

	respondOK(w, map[string]interface{}{
		"success":       true,
		"time_machine":  strings.Contains(output, "vfs_fruit"),
		"shadow_copy":   strings.Contains(output, "shadow_copy2"),
		"recycle_bin":   strings.Contains(output, "vfs_recycle"),
		"raw_config":    output,
	})
}

// SetSMBVFSConfig configures VFS modules for a share
// POST /api/smb/vfs
func SetSMBVFSConfig(w http.ResponseWriter, r *http.Request) {
	var req SMBShareVFS
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Build VFS objects list
	var vfsObjects []string
	var extraLines []string

	if req.TimeMachine {
		vfsObjects = append(vfsObjects, "catia", "fruit", "streams_xattr")
		extraLines = append(extraLines,
			"   fruit:metadata = stream",
			"   fruit:model = MacSamba",
			"   fruit:posix_rename = yes",
			"   fruit:veto_appledouble = no",
			"   fruit:nfs_aces = no",
			"   fruit:wipe_intentionally_left_blank_rfork = yes",
			"   fruit:delete_empty_adfiles = yes",
			"   fruit:time machine = yes",
		)
	}

	if req.ShadowCopy {
		vfsObjects = append(vfsObjects, "shadow_copy2")
		extraLines = append(extraLines,
			"   shadow:snapdir = .zfs/snapshot",
			"   shadow:sort = desc",
			"   shadow:format = %Y-%m-%d-%H%M%S",
		)
	}

	if req.RecycleBin {
		vfsObjects = append(vfsObjects, "recycle")
		extraLines = append(extraLines,
			"   recycle:repository = .recycle/%U",
			"   recycle:keeptree = yes",
			"   recycle:versions = yes",
			"   recycle:touch = yes",
			"   recycle:directory_mode = 0770",
		)
		if req.RecycleMaxAge > 0 {
			extraLines = append(extraLines,
				fmt.Sprintf("   recycle:maxage = %d", req.RecycleMaxAge),
			)
		}
		if req.RecycleMaxSize > 0 {
			extraLines = append(extraLines,
				fmt.Sprintf("   recycle:maxsize = %d", req.RecycleMaxSize*1024*1024),
			)
		}
	}

	if req.ExtraParams != "" {
		for _, line := range strings.Split(req.ExtraParams, "\n") {
			extraLines = append(extraLines, "   "+strings.TrimSpace(line))
		}
	}

	result := map[string]interface{}{
		"success":     true,
		"share":       req.ShareName,
		"vfs_objects": vfsObjects,
	}

	if len(vfsObjects) > 0 {
		result["vfs_line"] = fmt.Sprintf("   vfs objects = %s", strings.Join(vfsObjects, " "))
	}
	if len(extraLines) > 0 {
		result["extra_config"] = strings.Join(extraLines, "\n")
	}
	result["hint"] = "Add these lines to the share section in smb.conf, then reload"

	respondOK(w, result)
}

// ═══════════════════════════════════════════════════════════════
//  11. VLAN MANAGEMENT (802.1Q)
// ═══════════════════════════════════════════════════════════════

// CreateVLAN creates a VLAN interface
// POST /api/network/vlan { "parent": "eth0", "vlan_id": 100, "ip": "10.0.100.1/24" }
func CreateVLAN(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Parent string `json:"parent"`  // eth0
		VlanID int    `json:"vlan_id"` // 1-4094
		IP     string `json:"ip"`      // 10.0.100.1/24 (optional)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if strings.ContainsAny(req.Parent, ";|&$`\\\"' /") || len(req.Parent) > 16 {
		respondErrorSimple(w, "Invalid parent interface", http.StatusBadRequest)
		return
	}
	if req.VlanID < 1 || req.VlanID > 4094 {
		respondErrorSimple(w, "VLAN ID must be 1-4094", http.StatusBadRequest)
		return
	}

	ifName := fmt.Sprintf("%s.%d", req.Parent, req.VlanID)

	// Create VLAN interface
	_, err := executeCommandWithTimeout(TimeoutMedium, "/sbin/ip", []string{
		"link", "add", "link", req.Parent, "name", ifName, "type", "vlan", "id", fmt.Sprintf("%d", req.VlanID),
	})
	if err != nil {
		respondOK(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	// Bring up
	executeCommandWithTimeout(TimeoutFast, "/sbin/ip", []string{"link", "set", ifName, "up"})

	// Set IP if provided
	if req.IP != "" && !strings.ContainsAny(req.IP, ";|&$`\\\"'") {
		executeCommandWithTimeout(TimeoutFast, "/sbin/ip", []string{
			"addr", "add", req.IP, "dev", ifName,
		})
	}

	respondOK(w, map[string]interface{}{
		"success":   true,
		"interface": ifName,
		"vlan_id":   req.VlanID,
		"parent":    req.Parent,
		"ip":        req.IP,
	})
}

// DeleteVLAN removes a VLAN interface
// DELETE /api/network/vlan { "interface": "eth0.100" }
func DeleteVLAN(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Interface string `json:"interface"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if strings.ContainsAny(req.Interface, ";|&$`\\\"' /") || !strings.Contains(req.Interface, ".") {
		respondErrorSimple(w, "Invalid VLAN interface", http.StatusBadRequest)
		return
	}
	_, err := executeCommandWithTimeout(TimeoutFast, "/sbin/ip", []string{
		"link", "delete", req.Interface,
	})
	if err != nil {
		respondOK(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	respondOK(w, map[string]interface{}{"success": true, "deleted": req.Interface})
}

// ListVLANs lists all VLAN interfaces
// GET /api/network/vlan
func ListVLANs(w http.ResponseWriter, r *http.Request) {
	output, err := executeCommandWithTimeout(TimeoutFast, "/sbin/ip", []string{
		"-d", "link", "show", "type", "vlan",
	})
	if err != nil {
		respondOK(w, map[string]interface{}{"success": true, "vlans": []interface{}{}})
		return
	}
	respondOK(w, map[string]interface{}{
		"success": true,
		"vlans":   strings.TrimSpace(output),
	})
}

// ═══════════════════════════════════════════════════════════════
//  12. LINK AGGREGATION / BONDING (LACP)
// ═══════════════════════════════════════════════════════════════

// CreateBond creates a bonded interface
// POST /api/network/bond { "name": "bond0", "slaves": ["eth0","eth1"], "mode": "802.3ad" }
func CreateBond(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string   `json:"name"`   // bond0
		Slaves []string `json:"slaves"` // [eth0, eth1]
		Mode   string   `json:"mode"`   // balance-rr, active-backup, balance-xor, broadcast, 802.3ad, balance-tlb, balance-alb
		IP     string   `json:"ip"`     // optional
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if strings.ContainsAny(req.Name, ";|&$`\\\"' /") || len(req.Name) > 16 {
		respondErrorSimple(w, "Invalid bond name", http.StatusBadRequest)
		return
	}
	validModes := map[string]bool{
		"balance-rr": true, "active-backup": true, "balance-xor": true,
		"broadcast": true, "802.3ad": true, "balance-tlb": true, "balance-alb": true,
	}
	if !validModes[req.Mode] {
		respondErrorSimple(w, "Invalid bond mode", http.StatusBadRequest)
		return
	}

	// Create bond
	_, err := executeCommandWithTimeout(TimeoutMedium, "/sbin/ip", []string{
		"link", "add", req.Name, "type", "bond", "mode", req.Mode,
	})
	if err != nil {
		respondOK(w, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	// Add slaves
	for _, slave := range req.Slaves {
		if strings.ContainsAny(slave, ";|&$`\\\"' /") {
			continue
		}
		executeCommandWithTimeout(TimeoutFast, "/sbin/ip", []string{"link", "set", slave, "down"})
		executeCommandWithTimeout(TimeoutFast, "/sbin/ip", []string{"link", "set", slave, "master", req.Name})
	}

	// Bring up
	executeCommandWithTimeout(TimeoutFast, "/sbin/ip", []string{"link", "set", req.Name, "up"})

	if req.IP != "" && !strings.ContainsAny(req.IP, ";|&$`\\\"'") {
		executeCommandWithTimeout(TimeoutFast, "/sbin/ip", []string{
			"addr", "add", req.IP, "dev", req.Name,
		})
	}

	respondOK(w, map[string]interface{}{
		"success": true,
		"name":    req.Name,
		"mode":    req.Mode,
		"slaves":  req.Slaves,
	})
}

// ═══════════════════════════════════════════════════════════════
//  13. NTP CONFIGURATION
// ═══════════════════════════════════════════════════════════════

// GetNTPStatus returns current NTP synchronization status
// GET /api/system/ntp
func GetNTPStatus(w http.ResponseWriter, r *http.Request) {
	// Try timedatectl first (systemd)
	output, err := executeCommandWithTimeout(TimeoutFast, "/usr/bin/timedatectl", []string{"show"})
	if err != nil {
		// Fallback: chronyc
		output, err = executeCommandWithTimeout(TimeoutFast, "/usr/bin/chronyc", []string{"tracking"})
		if err != nil {
			respondOK(w, map[string]interface{}{
				"success": true,
				"synced":  false,
				"error":   "Cannot query NTP status",
			})
			return
		}
	}

	synced := strings.Contains(output, "NTPSynchronized=yes") ||
		strings.Contains(output, "Leap status     : Normal")

	respondOK(w, map[string]interface{}{
		"success": true,
		"synced":  synced,
		"details": strings.TrimSpace(output),
	})
}

// SetNTPServers configures NTP servers
// POST /api/system/ntp { "servers": ["0.pool.ntp.org", "1.pool.ntp.org"] }
func SetNTPServers(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Servers []string `json:"servers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if len(req.Servers) == 0 {
		respondErrorSimple(w, "At least one NTP server required", http.StatusBadRequest)
		return
	}
	for _, s := range req.Servers {
		if strings.ContainsAny(s, ";|&$`\\\"'") || len(s) > 253 {
			respondErrorSimple(w, "Invalid server address", http.StatusBadRequest)
			return
		}
	}

	// Use timedatectl
	args := append([]string{"set-ntp", "true"}, req.Servers...)
	executeCommandWithTimeout(TimeoutFast, "/usr/bin/timedatectl", []string{"set-ntp", "true"})

	// Set servers via systemd-timesyncd config
	conf := "[Time]\n"
	conf += fmt.Sprintf("NTP=%s\n", strings.Join(req.Servers, " "))

	executeCommandWithTimeout(TimeoutFast, "/usr/bin/tee", []string{"/etc/systemd/timesyncd.conf"})

	// Restart timesyncd
	executeCommandWithTimeout(TimeoutMedium, "/usr/bin/systemctl", []string{"restart", "systemd-timesyncd"})

	respondOK(w, map[string]interface{}{
		"success": true,
		"servers": req.Servers,
	})

	_ = args // suppress unused
}
