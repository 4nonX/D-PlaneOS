package handlers

import (
	"net/http"
	"os/exec"
	"regexp"
)

// HandleDockerLogs returns logs for a specific container
// GET /api/docker/logs?container=NAME&lines=100
func (h *DockerHandler) ContainerLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	containerName := r.URL.Query().Get("container")
	if containerName == "" {
		respondErrorSimple(w, "container parameter required", http.StatusBadRequest)
		return
	}

	// Validate container name (alphanumeric, dash, underscore, dot)
	validName := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)
	if !validName.MatchString(containerName) {
		respondErrorSimple(w, "Invalid container name", http.StatusBadRequest)
		return
	}

	lines := r.URL.Query().Get("lines")
	if lines == "" {
		lines = "200"
	}
	// Validate lines is a number
	validLines := regexp.MustCompile(`^\d{1,5}$`)
	if !validLines.MatchString(lines) {
		lines = "200"
	}

	output, err := cmdutil.RunMedium("/usr/bin/docker", "logs", "--tail", lines, containerName)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
			"logs":    string(output),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"container": containerName,
		"logs":      string(output),
	})
}
