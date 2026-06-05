package ha

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
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
//
// CRITICAL: Do NOT call sync() before rebooting. If pool export is stuck in
// D-state (uninterruptible kernel sleep on a hung bus or SCSI device), sync()
// will join the same D-state queue and this function will never reach reboot.
// The -f flag on reboot already instructs the kernel to skip sync and reboot
// immediately. Calling sync() defeats this entirely.
func ForceSelfReboot() {
	// Lock this goroutine to its OS thread so the reboot syscall is issued
	// from a thread that cannot be pre-empted or migrated by the Go scheduler
	// even if other goroutines are blocked in cgo D-state calls.
	runtime.LockOSThread()

	log.Printf("HA STONITH: SELF-REBOOT - pool export failed; rebooting to prevent split-brain data corruption")

	// Log to syslog. Use a non-blocking fire-and-forget; if syslog is
	// unavailable (e.g. hung filesystem) we must not block here.
	go exec.Command("logger", "-t", "dplaneos-ha", //nolint:errcheck
		"STONITH: self-reboot initiated - pool export timed out or failed").Run()

	// Primary: direct kernel syscall (Linux: syscall.Reboot / LINUX_REBOOT_CMD_RESTART).
	// Implemented in standby_reboot_linux.go. Bypasses all userspace - no fork,
	// no exec, no filesystem I/O. Most reliable path under OS pressure.
	if err := kernelReboot(); err != nil {
		// Fallback: exec reboot -f. The -f flag explicitly skips sync.
		// DO NOT add sync() before this - see function comment above.
		log.Printf("HA STONITH: kernel reboot syscall failed (%v); falling back to reboot -f", err)
		exec.Command("reboot", "-f").Run() //nolint:errcheck
		exec.Command("systemctl", "reboot", "--force").Run() //nolint:errcheck
	}

	// If somehow still running (e.g. reboot syscall was vetoed by a security
	// module), block indefinitely to prevent any further writes to shared storage.
	log.Printf("HA STONITH: WARNING - reboot did not complete; blocking all goroutines to prevent storage writes")
	select {}
}
