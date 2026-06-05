//go:build !libzfs

package libzfs

import "log"

func init() {
	log.Printf("WARNING: dplaned built without -tags libzfs; ZFS operations use subprocess fallback. " +
		"This is expected in CI and development. Production NixOS builds use mkDaemonCGO which sets -tags libzfs.")
}
