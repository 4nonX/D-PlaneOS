package handlers

import (
	"dplaned/internal/cmdutil"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
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
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Disks      []string `json:"disks"`
	TotalSize  string   `json:"total_size"`
	UsableSize string   `json:"usable_size"`
	Redundancy string   `json:"redundancy"`
}

type blockDevice struct {
	Name       string        `json:"name"`
	Size       string        `json:"size"`
	Type       string        `json:"type"`
	Model      string        `json:"model"`
	Serial     string        `json:"serial"`
	MountPoint string        `json:"mountpoint"`
	Children   []blockDevice `json:"children,omitempty"`
}

func HandleDiskDiscovery(w http.ResponseWriter, r *http.Request) {
	disks, err := discoverDisks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"disks":       disks,
		"suggestions": generatePoolSuggestions(disks),
	})
}

func discoverDisks() ([]DiskInfo, error) {
	lsblkOut, err := cmdutil.RunFast("lsblk", "-J", "-o", "NAME,SIZE,TYPE,MODEL,SERIAL,MOUNTPOINT")
	if err != nil {
		return nil, err
	}

	var result struct {
		BlockDevices []blockDevice `json:"blockdevices"`
	}

	if err := json.Unmarshal(lsblkOut, &result); err != nil {
		return nil, err
	}

	var disks []DiskInfo
	for _, dev := range result.BlockDevices {
		if dev.Type != "disk" {
			continue
		}

		// Skip system disks and any disks with mounted partitions
		inUse := hasMountPoint(dev) || isInZFSPool(dev.Name)

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

func hasMountPoint(dev blockDevice) bool {
	if strings.TrimSpace(dev.MountPoint) != "" {
		return true
	}
	for _, child := range dev.Children {
		if hasMountPoint(child) {
			return true
		}
	}
	return false
}

func isInZFSPool(diskName string) bool {
	zpoolOut, err := cmdutil.RunFast("zpool", "status", "-P")
	if err != nil {
		return false
	}

	return diskNameInZpoolStatus(string(zpoolOut), diskName)
}

func diskNameInZpoolStatus(status, diskName string) bool {
	if diskName == "" {
		return false
	}
	pattern := regexp.MustCompile(`(^|[^[:alnum:]])` + regexp.QuoteMeta(diskName) + `(p?[0-9]+)?([^[:alnum:]]|$)`)
	return pattern.MatchString(status)
}

func detectDiskType(name string) string {
	if strings.HasPrefix(name, "nvme") {
		return "NVMe"
	}

	rotData, err := os.ReadFile("/sys/block/" + name + "/queue/rotational")
	if err != nil {
		return "Unknown"
	}

	if strings.TrimSpace(string(rotData)) == "0" {
		return "SSD"
	}

	return "HDD"
}

func generatePoolSuggestions(disks []DiskInfo) []PoolSuggestion {
	var suggestions []PoolSuggestion

	var available []DiskInfo
	for _, disk := range disks {
		if !disk.InUse {
			available = append(available, disk)
		}
	}

	if len(available) == 0 {
		return suggestions
	}

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

	if len(available) >= 3 {
		var diskNames []string
		numDisks := 3
		if len(available) >= 4 {
			numDisks = 4
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

	if len(available) >= 4 {
		var diskNames []string
		numDisks := len(available)
		if numDisks > 6 {
			numDisks = 6
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

	if len(available) >= 5 {
		var diskNames []string
		for i := 0; i < len(available); i++ {
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

	if request.Name == "" {
		http.Error(w, "pool name is required", http.StatusBadRequest)
		return
	}
	if len(request.Disks) == 0 {
		http.Error(w, "at least one disk is required", http.StatusBadRequest)
		return
	}

	args := []string{"create", "-f", request.Name}

	switch request.Type {
	case "", "Single":
		// stripe/single vdev, no extra argument
	case "Mirror":
		args = append(args, "mirror")
	case "RAID-Z1":
		args = append(args, "raidz")
	case "RAID-Z2":
		args = append(args, "raidz2")
	case "RAID-Z3":
		args = append(args, "raidz3")
	default:
		http.Error(w, "invalid pool type", http.StatusBadRequest)
		return
	}

	args = append(args, request.Disks...)

	output, err := cmdutil.RunSlow("zpool", args...)

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

// HandlePoolImportScan lists pools available for import
func HandlePoolImportScan(w http.ResponseWriter, r *http.Request) {
	output, err := cmdutil.RunSlow("zpool", "import")
	if err != nil {
		// "no pools available to import" is not an error
		if strings.Contains(string(output), "no pools available") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"pools": []interface{}{},
			})
			return
		}
	}

	// Parse zpool import output into structured data
	type ImportablePool struct {
		Name   string `json:"name"`
		ID     string `json:"id"`
		State  string `json:"state"`
		Status string `json:"status"`
	}

	var pools []ImportablePool
	var current *ImportablePool

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "pool:") {
			if current != nil {
				pools = append(pools, *current)
			}
			current = &ImportablePool{
				Name: strings.TrimSpace(strings.TrimPrefix(line, "pool:")),
			}
		} else if current != nil {
			if strings.HasPrefix(line, "id:") {
				current.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			} else if strings.HasPrefix(line, "state:") {
				current.State = strings.TrimSpace(strings.TrimPrefix(line, "state:"))
			} else if strings.HasPrefix(line, "status:") {
				current.Status = strings.TrimSpace(strings.TrimPrefix(line, "status:"))
			}
		}
	}
	if current != nil {
		pools = append(pools, *current)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pools": pools,
	})
}

// HandlePoolImport imports an existing ZFS pool
func HandlePoolImport(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name  string `json:"name"`
		Force bool   `json:"force"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if request.Name == "" {
		http.Error(w, "pool name is required", http.StatusBadRequest)
		return
	}

	// Validate pool name (alphanumeric, hyphens, underscores only)
	if !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`).MatchString(request.Name) {
		http.Error(w, "invalid pool name", http.StatusBadRequest)
		return
	}

	args := []string{"import"}
	if request.Force {
		args = append(args, "-f")
	}
	args = append(args, request.Name)

	output, err := cmdutil.RunSlow("zpool", args...)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": strings.TrimSpace(string(output)),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "imported",
		"name":   request.Name,
	})
}

// HandlePoolExport cleanly exports a ZFS pool
func HandlePoolExport(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name  string `json:"name"`
		Force bool   `json:"force"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if request.Name == "" {
		http.Error(w, "pool name is required", http.StatusBadRequest)
		return
	}

	if !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`).MatchString(request.Name) {
		http.Error(w, "invalid pool name", http.StatusBadRequest)
		return
	}

	args := []string{"export"}
	if request.Force {
		args = append(args, "-f")
	}
	args = append(args, request.Name)

	output, err := cmdutil.RunSlow("zpool", args...)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": strings.TrimSpace(string(output)),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "exported",
		"name":   request.Name,
	})
}

// HandlePoolDestroy destroys a ZFS pool (requires confirmation via body)
func HandlePoolDestroy(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name    string `json:"name"`
		Confirm string `json:"confirm"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if request.Name == "" {
		http.Error(w, "pool name is required", http.StatusBadRequest)
		return
	}

	if !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`).MatchString(request.Name) {
		http.Error(w, "invalid pool name", http.StatusBadRequest)
		return
	}

	// Require explicit confirmation string matching pool name
	if request.Confirm != request.Name {
		http.Error(w, "confirmation must match pool name exactly", http.StatusBadRequest)
		return
	}

	output, err := cmdutil.RunSlow("zpool", "destroy", "-f", request.Name)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": strings.TrimSpace(string(output)),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "destroyed",
		"name":   request.Name,
	})
}
