// error_guide.go - Plain-English error enrichment for the top operational failure modes.
//
// The system produces correct HTTP status codes and machine-readable error fields.
// This layer adds a human-readable "guide" field with a plain-English next step
// aimed at the operator or support technician reading the response body.
//
// Apply respondGuided() in place of respondErrorSimple() / respondJSON() when the
// error state has a well-understood resolution path. The "guide" field is surfaced
// in the UI's error toast and in the support bundle.

package handlers

import (
	"fmt"
	"net/http"
)

// guidedError is the standard enriched error response body.
type guidedError struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`            // technical description
	Guide   string `json:"guide"`            // plain-English next step
	Code    string `json:"code,omitempty"`   // machine-readable code for UI routing
	Action  string `json:"action,omitempty"` // optional UI action hint
}

func respondGuided(w http.ResponseWriter, status int, e guidedError) {
	respondJSON(w, status, e)
}

// ─── Auth ─────────────────────────────────────────────────────────────────────

func guidedAAL2Required(w http.ResponseWriter) {
	respondGuided(w, http.StatusForbidden, guidedError{
		Error:  "This operation requires two-factor authentication.",
		Guide:  "Your session was authenticated with a password only. To perform this operation, enable TOTP under Settings > Security > Two-Factor Authentication, then log out and log in again.",
		Code:   "aal2_required",
		Action: "enable_totp",
	})
}

func guidedMustChangePassword(w http.ResponseWriter) {
	respondGuided(w, http.StatusForbidden, guidedError{
		Error:  "Your password must be changed before you can perform other operations.",
		Guide:  "An administrator has set a temporary password for your account. Go to Settings > Account > Change Password to set a new password.",
		Code:   "must_change_password",
		Action: "change_password",
	})
}

func guidedSessionExpired(w http.ResponseWriter) {
	respondGuided(w, http.StatusUnauthorized, guidedError{
		Error:  "Your session has expired or is invalid.",
		Guide:  "Please log in again. If you were recently inactive, sessions expire after 24 hours.",
		Code:   "session_expired",
		Action: "login",
	})
}

// ─── ZFS Quota ────────────────────────────────────────────────────────────────

func guidedQuotaBelowUsage(w http.ResponseWriter, dataset, quota, current string) {
	respondGuided(w, http.StatusBadRequest, guidedError{
		Error: fmt.Sprintf("The quota %s is below the dataset's current usage %s.", quota, current),
		Guide: fmt.Sprintf(
			"Dataset %q currently has %s in use (compressed). Setting a quota below this would make the dataset immediately read-only. "+
				"Set the quota above %s, or run 'zfs get referenced %s' to see the exact compressed usage. "+
				"If compression is enabled, logical usage may be higher than the referenced value shown.",
			dataset, current, current, dataset),
		Code: "quota_below_usage",
	})
}

// ─── Pool destroy ─────────────────────────────────────────────────────────────

func guidedPoolDestroyBlocked(w http.ResponseWriter, pool string, deps []string) {
	guide := fmt.Sprintf(
		"Pool %q has %d active service(s) using it. Stop them first:\n", pool, len(deps))
	for _, d := range deps {
		guide += fmt.Sprintf("  - %s\n", d)
	}
	guide += "After stopping all services, retry the destroy. If a service entry is stuck and cannot be removed, use force=true to bypass the soft check."
	respondGuided(w, http.StatusConflict, guidedError{
		Error: fmt.Sprintf("Pool %q cannot be destroyed while services are using it.", pool),
		Guide: guide,
		Code:  "pool_has_dependencies",
	})
}

// ─── HA blocked ──────────────────────────────────────────────────────────────

func guidedHADisabledReasons(w http.ResponseWriter, reasons []string) {
	guide := "HA auto-failover is currently suppressed. Reasons:\n"
	for _, r := range reasons {
		switch r {
		case "NO_FENCING_CONFIGURED":
			guide += "  - No fencing method configured. Add an IPMI BMC or PDU under Settings > HA > Fencing.\n"
		case "NO_WITNESS_REACHABLE":
			guide += "  - The quorum witness is unreachable. Check network connectivity to the witness node, or disable witness quorum if this is expected.\n"
		case "HYSTERESIS_ACTIVE":
			guide += "  - A recent failover is suppressing auto-failover for 60 minutes to prevent flapping. Use POST /api/ha/clear_fault if the root cause is resolved.\n"
		case "SUBORDINATE_MODE":
			guide += "  - This node booted with stale data and is still catching up. Wait for the sync to complete, or use POST /api/ha/clear_fault if you have verified data integrity manually.\n"
		case "MAINTENANCE_MODE":
			guide += "  - Maintenance mode is active. Use POST /api/ha/maintenance with seconds=0 to disable it.\n"
		default:
			guide += fmt.Sprintf("  - %s\n", r)
		}
	}
	respondGuided(w, http.StatusServiceUnavailable, guidedError{
		Error: "HA auto-failover is disabled.",
		Guide: guide,
		Code:  "ha_disabled",
	})
}

// ─── Persist mount ────────────────────────────────────────────────────────────

// guidedPersistMissing is called from the health endpoint if /persist is not
// writable. The system will halt shortly; this response gives the operator
// time to read the console before the halt fires.
func guidedPersistMissing(w http.ResponseWriter) {
	respondGuided(w, http.StatusServiceUnavailable, guidedError{
		Error: "The /persist partition is not mounted or not writable.",
		Guide: "DPlaneOS requires /persist to be mounted before it can operate. " +
			"The system will halt to prevent data loss. " +
			"Boot from the installer ISO, run 'fsck.ext4 -f /dev/sdX2' on the persist partition, " +
			"then reboot. If /persist is permanently lost, follow the Complete Database Reset " +
			"procedure in the RECOVERY documentation.",
		Code: "persist_missing",
	})
}
