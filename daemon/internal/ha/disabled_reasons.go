package ha

import "time"

// DisabledReason is a machine-readable code explaining why automatic HA
// failover is non-functional. Mirrors TrueNAS DisabledReasonsEnum from
// plugins/failover_/enums.py - surfaced directly to the UI so operators
// see "firmware versions differ between nodes" rather than "HA is broken."
type DisabledReason string

const (
	// ReasonNoFencingConfigured: no IPMI, PDU, or SBD fencing method enabled.
	ReasonNoFencingConfigured DisabledReason = "NO_FENCING_CONFIGURED"

	// ReasonNoWitnessReachable: witness probe failed; node may be isolated.
	ReasonNoWitnessReachable DisabledReason = "NO_WITNESS_REACHABLE"

	// ReasonHysteresisActive: within the 60-minute flap-suppression window.
	ReasonHysteresisActive DisabledReason = "HYSTERESIS_ACTIVE"

	// ReasonSubordinateMode: this node is still catching up stale data.
	ReasonSubordinateMode DisabledReason = "SUBORDINATE_MODE"

	// ReasonMaintenanceMode: operator has suspended automatic failover.
	ReasonMaintenanceMode DisabledReason = "MAINTENANCE_MODE"

	// ReasonFencingInProgress: a fencing sequence is already executing.
	ReasonFencingInProgress DisabledReason = "FENCING_IN_PROGRESS"

	// ReasonNoPeers: no peer nodes are registered in this cluster.
	ReasonNoPeers DisabledReason = "NO_PEERS"

	// ReasonAllPeersHealthy: steady state - no failover needed.
	ReasonAllPeersHealthy DisabledReason = "ALL_PEERS_HEALTHY"

	// ReasonVersionMismatch: daemon versions differ between nodes.
	ReasonVersionMismatch DisabledReason = "VERSION_MISMATCH"

	// ReasonClusterSecretMismatch: pre-shared secret mismatch; peer heartbeats rejected.
	ReasonClusterSecretMismatch DisabledReason = "CLUSTER_SECRET_MISMATCH"
)

// DisabledReasonDescription returns the human-readable explanation for a
// DisabledReason, suitable for display in the HA triage panel.
func DisabledReasonDescription(r DisabledReason) string {
	switch r {
	case ReasonNoFencingConfigured:
		return "No fencing method is configured (IPMI/BMC or PDU). Automatic failover cannot proceed without a way to guarantee the peer is offline before importing its pools."
	case ReasonNoWitnessReachable:
		return "The quorum witness is unreachable. This node may be network-isolated - promoting here could create a split-brain scenario."
	case ReasonHysteresisActive:
		return "Automatic failover is in the 60-minute cooldown window after the last event. Use POST /api/ha/clear_fault to override if this is not a flap."
	case ReasonSubordinateMode:
		return "This node is still catching up replicated data from the previous primary. Promoting while behind would serve stale data to clients."
	case ReasonMaintenanceMode:
		return "HA failover is suspended for maintenance. Use POST /api/ha/maintenance/end to re-enable."
	case ReasonFencingInProgress:
		return "A fencing operation is already in progress. Only one fencing sequence can execute at a time."
	case ReasonNoPeers:
		return "No peer nodes are registered in this cluster. Register a peer via POST /api/ha/peers."
	case ReasonAllPeersHealthy:
		return "All peer nodes are healthy. No failover needed."
	case ReasonVersionMismatch:
		return "Daemon versions differ between nodes. A mixed-version cluster may apply incompatible state transitions. Upgrade all nodes to the same version."
	case ReasonClusterSecretMismatch:
		return "The cluster pre-shared secret does not match between nodes. Heartbeats from the peer are being rejected."
	default:
		return string(r)
	}
}

// ClusterDisabledReasons evaluates the manager's current state and returns
// all reasons why automatic failover is non-functional. An empty slice means
// the cluster is healthy and failover-ready.
func (m *Manager) ClusterDisabledReasons() []DisabledReason {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.nodes) == 0 {
		return []DisabledReason{ReasonNoPeers}
	}

	// Check whether any peer is currently unreachable
	anyUnhealthy := false
	for _, n := range m.nodes {
		if n.State == StateUnreachable {
			anyUnhealthy = true
			break
		}
	}
	if !anyUnhealthy {
		return []DisabledReason{ReasonAllPeersHealthy}
	}

	var reasons []DisabledReason

	// No fencing configured
	fencingCfg, _ := GetFencingConfig(m.db)
	pduCfg, _ := GetPDUConfig(m.db)
	if !fencingCfg.Enable && !pduCfg.Enable {
		reasons = append(reasons, ReasonNoFencingConfigured)
	}

	// Maintenance mode
	if !m.maintenanceUntil.IsZero() && m.maintenanceUntil.After(time.Now()) {
		reasons = append(reasons, ReasonMaintenanceMode)
	}

	// Subordinate mode
	if m.subordinateMode {
		reasons = append(reasons, ReasonSubordinateMode)
	}

	// Hysteresis
	if !m.lastFailoverAt.IsZero() && time.Since(m.lastFailoverAt) < HysteresisWindow {
		reasons = append(reasons, ReasonHysteresisActive)
	}

	// Fencing already running
	if m.fencingInProgress {
		reasons = append(reasons, ReasonFencingInProgress)
	}

	// Witness unreachable (only relevant if configured)
	witnessCfg, witnessErr := GetWitnessConfig(m.db)
	if witnessErr == nil && witnessCfg.Enable && !canReachWitness(witnessCfg) {
		reasons = append(reasons, ReasonNoWitnessReachable)
	}

	// Version mismatch between nodes
	for _, n := range m.nodes {
		if n.Version != "" && m.version != "" && n.Version != m.version {
			reasons = append(reasons, ReasonVersionMismatch)
			break
		}
	}

	return reasons
}
