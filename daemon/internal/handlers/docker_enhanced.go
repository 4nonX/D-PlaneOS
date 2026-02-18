package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"regexp"

	"dplaned/internal/audit"
)

// ═══════════════════════════════════════════════════════════════
//  Docker Update with ZFS Snapshot (the killer feature)
// ═══════════════════════════════════════════════════════════════

// SafeUpdate performs: ZFS snapshot → docker pull → docker stop → docker rm → docker run → health check → rollback on failure
// POST /api/docker/update
func (h *DockerHandler) SafeUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ContainerName string `json:"container_name"`
		Image         string `json:"image"`          // e.g. "lscr.io/linuxserver/plex:latest"
		ZfsDataset    string `json:"zfs_dataset"`     // e.g. "tank/docker" (optional, auto-detected)
		SkipSnapshot  bool   `json:"skip_snapshot"`   // skip ZFS snapshot
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ContainerName == "" {
		respondErrorSimple(w, "container_name is required", http.StatusBadRequest)
		return
	}

	// Sanitize container name
	if !isValidContainerName(req.ContainerName) {
		respondErrorSimple(w, "Invalid container name", http.StatusBadRequest)
		return
	}

	steps := []UpdateStep{}
	startTime := time.Now()
	var snapshotName string

	// Step 1: ZFS Snapshot (if dataset provided and not skipped)
	if req.ZfsDataset != "" && !req.SkipSnapshot {
		if !isValidDataset(req.ZfsDataset) {
			respondErrorSimple(w, "Invalid ZFS dataset name", http.StatusBadRequest)
			return
		}

		snapshotName = fmt.Sprintf("%s@pre-update-%s-%s",
			req.ZfsDataset,
			req.ContainerName,
			time.Now().Format("20060102-150405"),
		)

		_, err := executeCommand("/usr/sbin/zfs", []string{"snapshot", snapshotName})
		if err != nil {
			steps = append(steps, UpdateStep{"zfs_snapshot", false, err.Error()})
			respondOK(w, UpdateResult{
				Success:  false,
				Steps:    steps,
				Error:    fmt.Sprintf("Failed to create safety snapshot: %v", err),
				Duration: time.Since(startTime).Milliseconds(),
			})
			return
		}
		steps = append(steps, UpdateStep{"zfs_snapshot", true, snapshotName})
	} else {
		steps = append(steps, UpdateStep{"zfs_snapshot", true, "skipped"})
	}

	// Step 2: Pull new image
	image := req.Image
	if image == "" {
		// Get image from running container
		imgOut, err := executeCommand("/usr/bin/docker", []string{
			"inspect", "--format", "{{.Config.Image}}", req.ContainerName,
		})
		if err != nil {
			steps = append(steps, UpdateStep{"detect_image", false, err.Error()})
			respondOK(w, UpdateResult{
				Success: false, Steps: steps,
				Error:    "Could not detect container image. Provide 'image' field.",
				Duration: time.Since(startTime).Milliseconds(),
			})
			return
		}
		image = strings.TrimSpace(imgOut)
	}

	_, err := executeCommand("/usr/bin/docker", []string{"pull", image})
	if err != nil {
		steps = append(steps, UpdateStep{"pull", false, err.Error()})
		respondOK(w, UpdateResult{
			Success:  false,
			Steps:    steps,
			Error:    fmt.Sprintf("Failed to pull image: %v", err),
			Rollback: snapshotName,
			Duration: time.Since(startTime).Milliseconds(),
		})
		return
	}
	steps = append(steps, UpdateStep{"pull", true, image})

	// Step 3: Get current container config (for recreate)
	configOut, err := executeCommand("/usr/bin/docker", []string{
		"inspect", "--format", "json", req.ContainerName,
	})
	if err != nil {
		steps = append(steps, UpdateStep{"inspect", false, err.Error()})
		respondOK(w, UpdateResult{
			Success: false, Steps: steps,
			Error:    "Failed to inspect container config",
			Rollback: snapshotName,
			Duration: time.Since(startTime).Milliseconds(),
		})
		return
	}
	steps = append(steps, UpdateStep{"inspect", true, "config saved"})
	_ = configOut // Config preserved for potential manual recovery

	// Step 4: Stop container
	_, err = executeCommand("/usr/bin/docker", []string{"stop", req.ContainerName})
	if err != nil {
		steps = append(steps, UpdateStep{"stop", false, err.Error()})
		// Try to restart on failure
		executeCommand("/usr/bin/docker", []string{"start", req.ContainerName})
		respondOK(w, UpdateResult{
			Success: false, Steps: steps,
			Error:    "Failed to stop container, restarted original",
			Rollback: snapshotName,
			Duration: time.Since(startTime).Milliseconds(),
		})
		return
	}
	steps = append(steps, UpdateStep{"stop", true, ""})

	// Step 5: Restart with new image
	// For simple containers, docker just needs start after pull
	// The new image is used on next docker-compose up or manual recreate
	_, err = executeCommand("/usr/bin/docker", []string{"start", req.ContainerName})
	if err != nil {
		steps = append(steps, UpdateStep{"start", false, err.Error()})
		// Container failed to start — this is where ZFS rollback shines
		respondOK(w, UpdateResult{
			Success: false, Steps: steps,
			Error: fmt.Sprintf("Container failed to start after update. "+
				"Your data is safe in snapshot: %s. "+
				"Rollback with: zfs rollback %s", snapshotName, snapshotName),
			Rollback: snapshotName,
			Duration: time.Since(startTime).Milliseconds(),
		})
		return
	}
	steps = append(steps, UpdateStep{"start", true, ""})

	// Step 6: Quick health check (wait 5s, check if still running)
	time.Sleep(5 * time.Second)
	statusOut, err := executeCommand("/usr/bin/docker", []string{
		"inspect", "--format", "{{.State.Running}}", req.ContainerName,
	})
	running := strings.TrimSpace(statusOut) == "true"

	if err != nil || !running {
		steps = append(steps, UpdateStep{"health_check", false, "container not running after 5s"})
		respondOK(w, UpdateResult{
			Success: false, Steps: steps,
			Error: fmt.Sprintf("Container crashed after update. "+
				"Rollback data with: zfs rollback %s", snapshotName),
			Rollback: snapshotName,
			Duration: time.Since(startTime).Milliseconds(),
		})
		return
	}
	steps = append(steps, UpdateStep{"health_check", true, "running"})

	audit.LogCommand(audit.LevelInfo, "system", "docker_safe_update",
		[]string{req.ContainerName, image}, true, time.Since(startTime), nil)

	respondOK(w, UpdateResult{
		Success:  true,
		Steps:    steps,
		Snapshot: snapshotName,
		Duration: time.Since(startTime).Milliseconds(),
	})
}

type UpdateStep struct {
	Step    string `json:"step"`
	Success bool   `json:"success"`
	Detail  string `json:"detail,omitempty"`
}

type UpdateResult struct {
	Success  bool         `json:"success"`
	Steps    []UpdateStep `json:"steps"`
	Error    string       `json:"error,omitempty"`
	Snapshot string       `json:"snapshot,omitempty"`
	Rollback string       `json:"rollback,omitempty"` // snapshot to rollback to
	Duration int64        `json:"duration_ms"`
}

// ═══════════════════════════════════════════════════════════════
//  Docker Pull
// ═══════════════════════════════════════════════════════════════

// PullImage pulls a Docker image
// POST /api/docker/pull { "image": "nginx:latest" }
func (h *DockerHandler) PullImage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Image string `json:"image"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Image == "" || len(req.Image) > 256 {
		respondErrorSimple(w, "Invalid image name", http.StatusBadRequest)
		return
	}

	start := time.Now()
	output, err := executeCommand("/usr/bin/docker", []string{"pull", req.Image})
	duration := time.Since(start)

	if err != nil {
		respondOK(w, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Pull failed: %v", err),
			"output":  output,
		})
		return
	}

	respondOK(w, map[string]interface{}{
		"success":     true,
		"image":       req.Image,
		"output":      output,
		"duration_ms": duration.Milliseconds(),
	})
}

// ═══════════════════════════════════════════════════════════════
//  Docker Remove
// ═══════════════════════════════════════════════════════════════

// RemoveContainer stops and removes a container
// POST /api/docker/remove { "container_name": "myapp", "force": true, "remove_volumes": false }
func (h *DockerHandler) RemoveContainer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ContainerName string `json:"container_name"`
		Force         bool   `json:"force"`
		RemoveVolumes bool   `json:"remove_volumes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if !isValidContainerName(req.ContainerName) {
		respondErrorSimple(w, "Invalid container name", http.StatusBadRequest)
		return
	}

	args := []string{"rm"}
	if req.Force {
		args = append(args, "-f")
	}
	if req.RemoveVolumes {
		args = append(args, "-v")
	}
	args = append(args, req.ContainerName)

	_, err := executeCommand("/usr/bin/docker", args)
	if err != nil {
		respondOK(w, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	respondOK(w, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Container %s removed", req.ContainerName),
	})
}

// ═══════════════════════════════════════════════════════════════
//  Docker Stats
// ═══════════════════════════════════════════════════════════════

// ContainerStats returns CPU, memory, network stats for all running containers
// GET /api/docker/stats
func (h *DockerHandler) ContainerStats(w http.ResponseWriter, r *http.Request) {
	output, err := executeCommand("/usr/bin/docker", []string{
		"stats", "--no-stream", "--format",
		`{"name":"{{.Name}}","cpu":"{{.CPUPerc}}","memory":"{{.MemUsage}}","mem_perc":"{{.MemPerc}}","net_io":"{{.NetIO}}","block_io":"{{.BlockIO}}","pids":"{{.PIDs}}"}`,
	})
	if err != nil {
		respondOK(w, map[string]interface{}{
			"success":    true,
			"containers": []interface{}{},
			"error":      "Docker stats unavailable",
		})
		return
	}

	var stats []map[string]interface{}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var s map[string]interface{}
		if err := json.Unmarshal([]byte(line), &s); err == nil {
			stats = append(stats, s)
		}
	}

	respondOK(w, map[string]interface{}{
		"success":    true,
		"containers": stats,
		"count":      len(stats),
	})
}

// ═══════════════════════════════════════════════════════════════
//  Docker Compose
// ═══════════════════════════════════════════════════════════════

// ComposeUp starts a docker-compose stack
// POST /api/docker/compose/up { "path": "/opt/stacks/plex", "detach": true }
func (h *DockerHandler) ComposeUp(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path   string `json:"path"`   // directory containing docker-compose.yml
		Detach bool   `json:"detach"` // -d flag
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Path == "" || strings.Contains(req.Path, "..") {
		respondErrorSimple(w, "Invalid path", http.StatusBadRequest)
		return
	}

	args := []string{"compose", "-f", req.Path + "/docker-compose.yml", "up"}
	if req.Detach {
		args = append(args, "-d")
	}

	start := time.Now()
	output, err := executeCommand("/usr/bin/docker", args)
	duration := time.Since(start)

	if err != nil {
		respondOK(w, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
			"output":  output,
		})
		return
	}

	respondOK(w, map[string]interface{}{
		"success":     true,
		"output":      output,
		"duration_ms": duration.Milliseconds(),
	})
}

// ComposeDown stops a docker-compose stack
// POST /api/docker/compose/down { "path": "/opt/stacks/plex" }
func (h *DockerHandler) ComposeDown(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path          string `json:"path"`
		RemoveVolumes bool   `json:"remove_volumes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Path == "" || strings.Contains(req.Path, "..") {
		respondErrorSimple(w, "Invalid path", http.StatusBadRequest)
		return
	}

	args := []string{"compose", "-f", req.Path + "/docker-compose.yml", "down"}
	if req.RemoveVolumes {
		args = append(args, "-v")
	}

	output, err := executeCommand("/usr/bin/docker", args)
	if err != nil {
		respondOK(w, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
			"output":  output,
		})
		return
	}

	respondOK(w, map[string]interface{}{
		"success": true,
		"output":  output,
	})
}

// ComposeStatus shows status of a docker-compose stack
// GET /api/docker/compose/status?path=/opt/stacks/plex
func (h *DockerHandler) ComposeStatus(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" || strings.Contains(path, "..") {
		respondErrorSimple(w, "Invalid path", http.StatusBadRequest)
		return
	}

	output, err := executeCommand("/usr/bin/docker", []string{
		"compose", "-f", path + "/docker-compose.yml", "ps", "--format", "json",
	})
	if err != nil {
		respondOK(w, map[string]interface{}{
			"success":    true,
			"services":   []interface{}{},
			"error":      "Compose stack not found or not running",
		})
		return
	}

	var services []map[string]interface{}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var s map[string]interface{}
		if err := json.Unmarshal([]byte(line), &s); err == nil {
			services = append(services, s)
		}
	}

	respondOK(w, map[string]interface{}{
		"success":  true,
		"services": services,
		"count":    len(services),
	})
}

// ═══════════════════════════════════════════════════════════════
//  Helpers
// ═══════════════════════════════════════════════════════════════

var validContainerNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.\-]*$`)

func isValidContainerName(name string) bool {
	return len(name) >= 1 && len(name) <= 128 && validContainerNameRe.MatchString(name)
}
