//go:build linux

package ha

import "golang.org/x/sys/unix"

// kernelReboot issues an immediate kernel reboot via direct syscall.
// This is the most reliable path under OS pressure: no fork, no exec,
// no filesystem I/O that could D-state on a hung ZFS pool.
func kernelReboot() error {
	// LINUX_REBOOT_CMD_RESTART: reboot immediately without syncing.
	// Equivalent to pressing the physical reset button.
	return unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART)
}
