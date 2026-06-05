//go:build !linux

package ha

import "fmt"

// kernelReboot is a no-op stub on non-Linux platforms (test builds on macOS/Windows).
func kernelReboot() error {
	return fmt.Errorf("kernel reboot not supported on this platform")
}
