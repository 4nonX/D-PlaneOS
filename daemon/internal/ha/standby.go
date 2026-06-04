package ha

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"dplaned/internal/cmdutil"
	"dplaned/internal/libzfs"
)

// ExportPoolTimeout is the maximum time allowed to export ZFS pools before
// the node force-reboots itself. Mirrors TrueNAS ZPOOL_EXPORT_TIMEOUT = 4s
// from plugins/failover_/event.py.
//
// If this node cannot yield its pools within the window, it kills itself
// rather than risk both nodes writing simultaneously. A split-brain where
// two nodes write to the same ZFS pool permanently corrupts pool metadata.
const ExportPoolTimeout = 4 * time.Second

// BecomeStandby transitions this node to standby by cleanly exporting all
// ZFS pools within ExportPoolTimeout. If the export does not complete in
// time, the node immediately force-reboots itself.
//
// Call this when:
//   - An admin manually initiates a planned failover
//   - Keepalived notifies this node it has lost the VIP (BACKUP state)
//
// This path is NOT used during unplanned failover (peer crash): in that case
// the surviving peer is the authoritative actor and this node was already
// fenced or is already down.
func BecomeStandby() error {
	log.Printf("HA STANDBY: Beginning graceful pool export (deadline: %v)", ExportPoolTimeout)

	done := make(chan error, 1)
	go func() {
		done <- exportAllPools()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("HA STANDBY: Pool export failed (%v) - force-rebooting to prevent split-brain", err)
			ForceSelfReboot()
			// Only reached in test environments where ForceSelfReboot is a no-op.
			return fmt.Errorf("pool export failed: %w", err)
		}
		log.Printf("HA STANDBY: All pools exported cleanly - node is safe to become standby")
		return nil

	case <-time.After(ExportPoolTimeout):
		log.Printf("HA STANDBY: Pool export timed out after %v - force-rebooting to prevent split-brain", ExportPoolTimeout)
		ForceSelfReboot()
		return fmt.Errorf("pool export timed out after %v", ExportPoolTimeout)
	}
}

// exportAllPools lists and exports every currently-imported ZFS pool.
func exportAllPools() error {
	out, err := cmdutil.RunFast("zpool_list", "list", "-H", "-o", "name")
	if err != nil {
		return fmt.Errorf("zpool list: %w", err)
	}

	for _, pool := range strings.Fields(strings.TrimSpace(string(out))) {
		log.Printf("HA STANDBY: Exporting pool %q", pool)
		if err := libzfs.PoolExport(pool, false); err != nil {
			return fmt.Errorf("export pool %q: %w", pool, err)
		}
		log.Printf("HA STANDBY: Pool %q exported", pool)
	}
	return nil
}

// ForceSelfReboot force-reboots this node unconditionally. This is the
// last-resort safety measure when graceful standby transition fails.
//
// A node that cannot cleanly export its pools MUST be considered unsafe.
// Rebooting is preferable to allowing it to continue and potentially write
// to a pool that the new primary has already imported.
func ForceSelfReboot() {
	log.Printf("HA STONITH: SELF-REBOOT - pool export failed; rebooting to prevent split-brain data corruption")

	// Log to syslog before reboot since this process will not continue.
	exec.Command("logger", "-t", "dplaneos-ha", //nolint:errcheck
		"STONITH: self-reboot initiated - pool export timed out or failed").Run()

	// Flush filesystem buffers, then reboot.
	exec.Command("sync").Run() //nolint:errcheck
	if err := exec.Command("reboot", "-f").Run(); err != nil {
		log.Printf("HA STONITH: reboot -f failed (%v); trying systemctl reboot", err)
		exec.Command("systemctl", "reboot", "--force").Run() //nolint:errcheck
	}

	// If somehow still running, block indefinitely to prevent any
	// further writes to shared storage.
	log.Printf("HA STONITH: WARNING - reboot did not complete; blocking to prevent storage writes")
	select {}
}
