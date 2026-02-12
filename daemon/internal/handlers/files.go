package handlers

import (
	"encoding/json"
	"net/http"

	"dplaned/internal/audit"
	"dplaned/internal/cmdutil"
	"dplaned/internal/security"
)

// CreateDirectory creates a directory
func CreateDirectory(w http.ResponseWriter, r *http.Request) {
	user := r.Header.Get("X-User")
	sessionID := r.Header.Get("X-Session-ID")

	if valid, _ := security.ValidateSession(sessionID, user); !valid {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Path string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	output, err := cmdutil.RunFast("mkdir", "-p", req.Path)

	audit.LogActivity(user, "directory_create", map[string]interface{}{
		"path":    req.Path,
		"success": err == nil,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"path":    req.Path,
		"output":  string(output),
	})
}

// DeletePath deletes a file or directory
func DeletePath(w http.ResponseWriter, r *http.Request) {
	user := r.Header.Get("X-User")
	sessionID := r.Header.Get("X-Session-ID")

	if valid, _ := security.ValidateSession(sessionID, user); !valid {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Path string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	output, err := cmdutil.RunFast("rm", "-rf", req.Path)

	audit.LogActivity(user, "path_delete", map[string]interface{}{
		"path":    req.Path,
		"success": err == nil,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"path":    req.Path,
		"output":  string(output),
	})
}

// ChangeOwnership changes file/directory ownership
func ChangeOwnership(w http.ResponseWriter, r *http.Request) {
	user := r.Header.Get("X-User")
	sessionID := r.Header.Get("X-Session-ID")

	if valid, _ := security.ValidateSession(sessionID, user); !valid {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Path  string `json:"path"`
		Owner string `json:"owner"`
		Group string `json:"group"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ownerGroup := req.Owner
	if req.Group != "" {
		ownerGroup = req.Owner + ":" + req.Group
	}

	output, err := cmdutil.RunFast("chown", ownerGroup, req.Path)

	audit.LogActivity(user, "ownership_change", map[string]interface{}{
		"path":    req.Path,
		"owner":   req.Owner,
		"group":   req.Group,
		"success": err == nil,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = output
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

// ChangePermissions changes file/directory permissions
func ChangePermissions(w http.ResponseWriter, r *http.Request) {
	user := r.Header.Get("X-User")
	sessionID := r.Header.Get("X-Session-ID")

	if valid, _ := security.ValidateSession(sessionID, user); !valid {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Path        string `json:"path"`
		Permissions string `json:"permissions"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	output, err := cmdutil.RunFast("chmod", req.Permissions, req.Path)

	audit.LogActivity(user, "permissions_change", map[string]interface{}{
		"path":        req.Path,
		"permissions": req.Permissions,
		"success":     err == nil,
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = output
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}
