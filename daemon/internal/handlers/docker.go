package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"dplaned/internal/audit"
	"dplaned/internal/security"
)

type DockerHandler struct{}

func NewDockerHandler() *DockerHandler {
	return &DockerHandler{}
}

func (h *DockerHandler) ListContainers(w http.ResponseWriter, r *http.Request) {
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
	output, err := executeCommand("/usr/bin/docker", []string{"ps", "-a", "--format", "{{json .}}"})
	duration := time.Since(start)

	audit.LogCommand(audit.LevelInfo, user, "docker_ps", nil, err == nil, duration, err)

	if err != nil {
		respondOK(w, CommandResponse{
			Success:  false,
			Error:    err.Error(),
			Duration: duration.Milliseconds(),
		})
		return
	}

	containers := parseDockerPS(output)

	respondOK(w, CommandResponse{
		Success:  true,
		Data:     containers,
		Duration: duration.Milliseconds(),
	})
}

func (h *DockerHandler) ContainerAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Action      string `json:"action"`      // start, stop, restart
		ContainerID string `json:"container_id"`
		SessionID   string `json:"session_id"`
		User        string `json:"user"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if !security.IsValidSessionToken(req.SessionID) {
		audit.LogSecurityEvent("Invalid session token", req.User, r.RemoteAddr)
		respondErrorSimple(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate action
	validActions := map[string]bool{"start": true, "stop": true, "restart": true, "pause": true, "unpause": true}
	if !validActions[req.Action] {
		respondErrorSimple(w, "Invalid action", http.StatusBadRequest)
		return
	}

	// Validate container ID format
	if !strings.HasPrefix(req.ContainerID, "") || len(req.ContainerID) < 3 || len(req.ContainerID) > 64 {
		respondErrorSimple(w, "Invalid container ID", http.StatusBadRequest)
		return
	}

	start := time.Now()
	output, err := executeCommand("/usr/bin/docker", []string{req.Action, req.ContainerID})
	duration := time.Since(start)

	audit.LogCommand(
		audit.LevelInfo,
		req.User,
		"docker_"+req.Action,
		[]string{req.ContainerID},
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

	respondOK(w, CommandResponse{
		Success:  true,
		Output:   output,
		Duration: duration.Milliseconds(),
	})
}

// Helper functions

func parseDockerPS(output string) []map[string]interface{} {
	var containers []map[string]interface{}
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		var container map[string]interface{}
		if err := json.Unmarshal([]byte(line), &container); err != nil {
			continue
		}

		containers = append(containers, container)
	}

	return containers
}
