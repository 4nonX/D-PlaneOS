package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

type DiskInfo struct {
	Name       string `json:"name"`
	Size       string `json:"size"`
	Type       string `json:"type"`
	Model      string `json:"model"`
	Serial     string `json:"serial"`
	InUse      bool   `json:"in_use"`
	MountPoint string `json:"mount_point,omitempty"`
}

type PoolSuggestion struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Disks       []string `json:"disks"`
	TotalSize   string   `json:"total_size"`
	UsableSize  string   `json:"usable_size"`
	Redundancy  string   `json:"redundancy"`
}

func HandleDiskDiscovery(w http.ResponseWriter, r *http.Request) {
	disks, err := discoverDisks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"disks": disks,
		"suggestions": generatePoolSuggestions(disks),
	})
}

func discoverDisks() ([]DiskInfo, error) {
	// Use lsblk to get disk information
	cmd := exec.Command("lsblk", "-J", "-o", "NAME,SIZE,TYPE,MODEL,SERIAL,MOUNTPOINT")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	
	var result struct {
		BlockDevices []struct {
			Name       string `json:"name"`
			Size       string `json:"size"`
			Type       string `json:"type"`
			Model      string `json:"model"`
			Serial     string `json:"serial"`
			MountPoint string `json:"mountpoint"`
		} `json:"blockdevices"`
	}
	
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}
	
	var disks []DiskInfo
	for _, dev := range result.BlockDevices {
		if dev.Type != "disk" {
			continue
		}
		
		// Skip system disk (mounted on /)
		inUse := dev.MountPoint != "" || isInZFSPool(dev.Name)
		
		disks = append(disks, DiskInfo{
			Name:       dev.Name,
			Size:       dev.Size,
			Type:       detectDiskType(dev.Name),
			Model:      dev.Model,
			Serial:     dev.Serial,
			InUse:      inUse,
			MountPoint: dev.MountPoint,
		})
	}
	
	return disks, nil
}

func isInZFSPool(diskName string) bool {
	cmd := exec.Command("zpool", "status")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	
	return strings.Contains(string(output), diskName)
}

func detectDiskType(name string) string {
	if strings.HasPrefix(name, "nvme") {
		return "NVMe"
	}
	
	// Check rotation rate
	cmd := exec.Command("cat", "/sys/block/"+name+"/queue/rotational")
	output, err := cmd.Output()
	if err != nil {
		return "Unknown"
	}
	
	if strings.TrimSpace(string(output)) == "0" {
		return "SSD"
	}
	
	return "HDD"
}

func generatePoolSuggestions(disks []DiskInfo) []PoolSuggestion {
	var suggestions []PoolSuggestion
	
	// Filter unused disks
	var available []DiskInfo
	for _, disk := range disks {
		if !disk.InUse {
			available = append(available, disk)
		}
	}
	
	if len(available) == 0 {
		return suggestions
	}
	
	// Suggestion 1: Single disk (no redundancy)
	if len(available) >= 1 {
		suggestions = append(suggestions, PoolSuggestion{
			Name:       "tank",
			Type:       "Single",
			Disks:      []string{available[0].Name},
			TotalSize:  available[0].Size,
			UsableSize: available[0].Size,
			Redundancy: "None - Data loss if disk fails",
		})
	}
	
	// Suggestion 2: Mirror (2 disks)
	if len(available) >= 2 {
		suggestions = append(suggestions, PoolSuggestion{
			Name:       "tank",
			Type:       "Mirror",
			Disks:      []string{available[0].Name, available[1].Name},
			TotalSize:  available[0].Size + " (mirrored)",
			UsableSize: available[0].Size,
			Redundancy: "1 disk failure",
		})
	}
	
	// Suggestion 3: RAID-Z1 (3+ disks)
	if len(available) >= 3 {
		var diskNames []string
		numDisks := 3
		if len(available) >= 4 {
			numDisks = 4 // Use 4 disks if available for better performance
		}
		
		for i := 0; i < numDisks && i < len(available); i++ {
			diskNames = append(diskNames, available[i].Name)
		}
		
		suggestions = append(suggestions, PoolSuggestion{
			Name:       "tank",
			Type:       "RAID-Z1",
			Disks:      diskNames,
			TotalSize:  fmt.Sprintf("%s x %d", available[0].Size, len(diskNames)),
			UsableSize: fmt.Sprintf("%s x %d", available[0].Size, len(diskNames)-1),
			Redundancy: "1 disk failure",
		})
	}
	
	// Suggestion 4: RAID-Z2 (4+ disks) - RECOMMENDED
	if len(available) >= 4 {
		var diskNames []string
		numDisks := len(available)
		if numDisks > 6 {
			numDisks = 6 // Cap at 6 for optimal RAID-Z2
		}
		
		for i := 0; i < numDisks; i++ {
			diskNames = append(diskNames, available[i].Name)
		}
		
		suggestions = append(suggestions, PoolSuggestion{
			Name:       "tank",
			Type:       "RAID-Z2",
			Disks:      diskNames,
			TotalSize:  fmt.Sprintf("%s x %d", available[0].Size, len(diskNames)),
			UsableSize: fmt.Sprintf("%s x %d", available[0].Size, len(diskNames)-2),
			Redundancy: "2 disk failures (Recommended)",
		})
	}
	
	// Suggestion 5: RAID-Z3 (5+ disks) - MAXIMUM PROTECTION
	if len(available) >= 5 {
		var diskNames []string
		numDisks := len(available)
		
		for i := 0; i < numDisks; i++ {
			diskNames = append(diskNames, available[i].Name)
		}
		
		suggestions = append(suggestions, PoolSuggestion{
			Name:       "tank",
			Type:       "RAID-Z3",
			Disks:      diskNames,
			TotalSize:  fmt.Sprintf("%s x %d", available[0].Size, len(diskNames)),
			UsableSize: fmt.Sprintf("%s x %d", available[0].Size, len(diskNames)-3),
			Redundancy: "3 disk failures (Maximum protection)",
		})
	}
	
	return suggestions
}

func HandlePoolCreate(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name  string   `json:"name"`
		Type  string   `json:"type"`
		Disks []string `json:"disks"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	// Build zpool create command
	args := []string{"create", "-f", request.Name}
	
	switch request.Type {
	case "Mirror":
		args = append(args, "mirror")
	case "RAID-Z1":
		args = append(args, "raidz")
	case "RAID-Z2":
		args = append(args, "raidz2")
	case "RAID-Z3":
		args = append(args, "raidz3")
	}
	
	args = append(args, request.Disks...)
	
	cmd := exec.Command("zpool", args...)
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": string(output),
		})
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "created",
		"name":   request.Name,
	})
}
