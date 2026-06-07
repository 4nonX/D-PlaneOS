package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"dplaned/internal/ha"
)

// NetworkWitnessHandler manages the network quorum witness configuration.
// The witness is a plain IP or URL (VPS, cloud metadata endpoint, DNS server)
// that both HA nodes independently probe. No software is installed on the target.
type NetworkWitnessHandler struct {
	db *sql.DB
}

func NewNetworkWitnessHandler(db *sql.DB) *NetworkWitnessHandler {
	return &NetworkWitnessHandler{db: db}
}

// GetConfig handles GET /api/ha/network-witness
func (h *NetworkWitnessHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := ha.GetNetworkWitnessConfig(h.db)
	if err != nil {
		respondErrorSimple(w, "Failed to load network witness config: "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"success": true, "config": cfg})
}

// SaveConfig handles POST /api/ha/network-witness
func (h *NetworkWitnessHandler) SaveConfig(w http.ResponseWriter, r *http.Request) {
	var req ha.NetworkWitnessConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErrorSimple(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := ha.SaveNetworkWitnessConfig(h.db, req); err != nil {
		respondErrorSimple(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Network witness configuration saved",
	})
}

// Probe handles POST /api/ha/network-witness/probe
// Runs a live connectivity check from this node to the configured witness.
// Call this from the UI to verify the witness is reachable before saving.
func (h *NetworkWitnessHandler) Probe(w http.ResponseWriter, r *http.Request) {
	cfg, err := ha.GetNetworkWitnessConfig(h.db)
	if err != nil {
		respondErrorSimple(w, "Failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Allow the caller to override target/method for a test probe
	var override struct {
		Target  string `json:"target"`
		Method  string `json:"method"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&override)
	}
	if override.Target != "" {
		cfg.Target = override.Target
		cfg.Enable = true
	}
	if override.Method != "" {
		cfg.Method = override.Method
	}

	result := ha.ProbeNetworkWitness(cfg)
	respondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"result":  result,
	})
}
