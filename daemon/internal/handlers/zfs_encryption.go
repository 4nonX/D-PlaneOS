package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"dplaned/internal/security"
)

type ZFSEncryptionHandler struct{}

func NewZFSEncryptionHandler() *ZFSEncryptionHandler {
	return &ZFSEncryptionHandler{}
}

type EncryptedDataset struct {
	Name          string `json:"name"`
	Encryption    string `json:"encryption"`
	KeyStatus     string `json:"keystatus"`
	KeyLocation   string `json:"keylocation"`
	KeyFormat     string `json:"keyformat"`
}

// ListEncryptedDatasets lists all encrypted ZFS datasets
func (h *ZFSEncryptionHandler) ListEncryptedDatasets(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("zfs", "list", "-H", "-o", "name,encryption,keystatus,keylocation,keyformat", "-t", "filesystem,volume")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list datasets: %v", err), http.StatusInternalServerError)
		return
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	datasets := make([]EncryptedDataset, 0)

	for _, line := range lines {
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			fields = strings.Fields(line)
		}
		if len(fields) >= 5 && fields[1] != "off" && fields[1] != "-" {
			datasets = append(datasets, EncryptedDataset{
				Name:        fields[0],
				Encryption:  fields[1],
				KeyStatus:   fields[2],
				KeyLocation: fields[3],
				KeyFormat:   fields[4],
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"datasets": datasets,
	})
}

// UnlockDataset unlocks an encrypted dataset
func (h *ZFSEncryptionHandler) UnlockDataset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dataset string `json:"dataset"`
		Key     string `json:"key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := security.ValidateDatasetName(req.Dataset); err != nil {
		http.Error(w, "Invalid dataset name: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Create temporary key file
	cmd := exec.Command("zfs", "load-key", req.Dataset)
	cmd.Stdin = strings.NewReader(req.Key)
	
	if output, err := cmd.CombinedOutput(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to unlock: %v - %s", err, string(output)),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// LockDataset locks an encrypted dataset
func (h *ZFSEncryptionHandler) LockDataset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dataset string `json:"dataset"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := security.ValidateDatasetName(req.Dataset); err != nil {
		http.Error(w, "Invalid dataset name: "+err.Error(), http.StatusBadRequest)
		return
	}

	cmd := exec.Command("zfs", "unload-key", req.Dataset)
	if output, err := cmd.CombinedOutput(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to lock: %v - %s", err, string(output)),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// CreateEncryptedDataset creates a new encrypted dataset
func (h *ZFSEncryptionHandler) CreateEncryptedDataset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		Encryption string `json:"encryption"`
		Key        string `json:"key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Encryption == "" {
		req.Encryption = "aes-256-gcm"
	}

	if err := security.ValidateDatasetName(req.Name); err != nil {
		http.Error(w, "Invalid dataset name: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Whitelist encryption algorithms
	validAlgos := map[string]bool{"aes-128-ccm": true, "aes-192-ccm": true, "aes-256-ccm": true, "aes-128-gcm": true, "aes-192-gcm": true, "aes-256-gcm": true}
	if !validAlgos[req.Encryption] {
		http.Error(w, "Invalid encryption algorithm", http.StatusBadRequest)
		return
	}

	cmd := exec.Command("zfs", "create", 
		"-o", fmt.Sprintf("encryption=%s", req.Encryption),
		"-o", "keyformat=passphrase",
		"-o", "keylocation=prompt",
		req.Name,
	)
	cmd.Stdin = strings.NewReader(req.Key + "\n" + req.Key + "\n")
	
	if output, err := cmd.CombinedOutput(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to create: %v - %s", err, string(output)),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// ChangeKey changes the encryption key
func (h *ZFSEncryptionHandler) ChangeKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dataset string `json:"dataset"`
		OldKey  string `json:"old_key"`
		NewKey  string `json:"new_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := security.ValidateDatasetName(req.Dataset); err != nil {
		http.Error(w, "Invalid dataset name: "+err.Error(), http.StatusBadRequest)
		return
	}

	cmd := exec.Command("zfs", "change-key", req.Dataset)
	cmd.Stdin = strings.NewReader(req.OldKey + "\n" + req.NewKey + "\n" + req.NewKey + "\n")
	
	if output, err := cmd.CombinedOutput(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to change key: %v - %s", err, string(output)),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
