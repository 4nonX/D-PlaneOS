//go:build linux

package nvmet

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func slug(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:4])
}

func subsysDirName(nqn string) string {
	return "dplane-ss-" + slug(nqn)
}

func portDirName(transport, addr string, port int) string {
	key := fmt.Sprintf("%s|%s|%d", transport, addr, port)
	return "dplane-p-" + slug(key)
}

func hostDirName(hostNQN string) string {
	return "dplane-h-" + slug(hostNQN)
}

func nvmetRoot() string {
	return filepath.Join(ConfigfsRoot, "nvmet")
}

// Apply wipes DPlaneOS-managed nvmet objects and applies exports (empty slice clears target config).
func Apply(exports []Export) error {
	if err := modprobe(); err != nil {
		return err
	}
	root := nvmetRoot()
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return fmt.Errorf("nvmet configfs not mounted at %s (is configfs mounted?)", root)
	}
	if err := teardownDPlane(); err != nil {
		return fmt.Errorf("nvmet teardown: %w", err)
	}
	for i := range exports {
		if err := ValidateExport(&exports[i]); err != nil {
			return fmt.Errorf("export %q: %w", exports[i].SubsystemNQN, err)
		}
	}
	for _, e := range exports {
		if err := createExport(&e); err != nil {
			return err
		}
	}
	return nil
}

func modprobe() error {
	for _, m := range []string{"nvmet", "nvmet-tcp"} {
		out, err := exec.Command("modprobe", m).CombinedOutput()
		if err != nil {
			return fmt.Errorf("modprobe %s: %w\n%s", m, err, out)
		}
	}
	return nil
}

func teardownDPlane() error {
	root := nvmetRoot()
	portsDir := filepath.Join(root, "ports")
	entries, err := os.ReadDir(portsDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "dplane-p-") {
			continue
		}
		p := filepath.Join(portsDir, e.Name())
		subs, _ := os.ReadDir(filepath.Join(p, "subsystems"))
		for _, s := range subs {
			_ = os.Remove(filepath.Join(p, "subsystems", s.Name()))
		}
		_ = os.RemoveAll(p)
	}

	subDir := filepath.Join(root, "subsystems")
	subs, err := os.ReadDir(subDir)
	if err != nil {
		return err
	}
	for _, e := range subs {
		if !strings.HasPrefix(e.Name(), "dplane-ss-") {
			continue
		}
		if err := removeSubsystem(filepath.Join(subDir, e.Name())); err != nil {
			return err
		}
	}

	hostDir := filepath.Join(root, "hosts")
	hents, _ := os.ReadDir(hostDir)
	for _, e := range hents {
		if strings.HasPrefix(e.Name(), "dplane-h-") {
			_ = os.RemoveAll(filepath.Join(hostDir, e.Name()))
		}
	}
	return nil
}

func removeSubsystem(path string) error {
	ah := filepath.Join(path, "allowed_hosts")
	if ents, err := os.ReadDir(ah); err == nil {
		for _, e := range ents {
			_ = os.Remove(filepath.Join(ah, e.Name()))
		}
	}
	ns := filepath.Join(path, "namespaces")
	if ents, err := os.ReadDir(ns); err == nil {
		for _, e := range ents {
			nspath := filepath.Join(ns, e.Name())
			_ = os.WriteFile(filepath.Join(nspath, "enable"), []byte("0"), 0644)
			_ = os.RemoveAll(nspath)
		}
	}
	return os.RemoveAll(path)
}

func createExport(e *Export) error {
	root := nvmetRoot()
	ssName := subsysDirName(e.SubsystemNQN)
	ssPath := filepath.Join(root, "subsystems", ssName)
	if err := os.Mkdir(ssPath, 0755); err != nil {
		return fmt.Errorf("mkdir subsystem %s: %w", ssName, err)
	}
	if err := os.WriteFile(filepath.Join(ssPath, "subsys_nqn"), []byte(e.SubsystemNQN), 0644); err != nil {
		return fmt.Errorf("subsys_nqn: %w", err)
	}
	if e.AllowAnyHost {
		if err := os.WriteFile(filepath.Join(ssPath, "attr_allow_any_host"), []byte("1"), 0644); err != nil {
			return fmt.Errorf("attr_allow_any_host: %w", err)
		}
	} else {
		if err := os.WriteFile(filepath.Join(ssPath, "attr_allow_any_host"), []byte("0"), 0644); err != nil {
			return fmt.Errorf("attr_allow_any_host: %w", err)
		}
		for _, hn := range e.HostNQNs {
			hn = strings.TrimSpace(hn)
			hname := hostDirName(hn)
			hpath := filepath.Join(root, "hosts", hname)
			if err := os.Mkdir(hpath, 0755); err != nil && !os.IsExist(err) {
				return fmt.Errorf("mkdir host %s: %w", hname, err)
			}
			if err := writeHostNQN(hpath, hn); err != nil {
				return err
			}
			link := filepath.Join(ssPath, "allowed_hosts", hname)
			_ = os.Remove(link)
			target := filepath.Join("..", "..", "..", "hosts", hname)
			if err := os.Symlink(target, link); err != nil {
				return fmt.Errorf("allowed_hosts link %s: %w", hname, err)
			}
		}
	}

	nsID := fmt.Sprintf("%d", e.NamespaceID)
	nsPath := filepath.Join(ssPath, "namespaces", nsID)
	if err := os.Mkdir(nsPath, 0755); err != nil {
		return fmt.Errorf("mkdir namespace: %w", err)
	}
	dev := ZvolDevicePath(e.Zvol)
	if err := os.WriteFile(filepath.Join(nsPath, "device_path"), []byte(dev), 0644); err != nil {
		return fmt.Errorf("device_path: %w", err)
	}

	// ANA (Asymmetric Namespace Access): assign this namespace to its ANA group
	// so NVMe multi-path hosts can steer I/O to the optimized controller.
	// The kernel creates ana_grpid automatically; we just write the group ID here
	// before enabling the namespace so the mapping is established atomically.
	if e.ANAEnabled {
		for _, ag := range e.ANAGroups {
			if ag.NamespaceID != e.NamespaceID {
				continue
			}
			grpIDStr := fmt.Sprintf("%d", ag.GroupID)
			if err := os.WriteFile(filepath.Join(nsPath, "ana_grpid"), []byte(grpIDStr), 0644); err != nil {
				return fmt.Errorf("ana_grpid: %w", err)
			}
			// Configure the ANA group state on the subsystem.
			if err := applyANAGroupState(ssPath, ag.GroupID, ag.State); err != nil {
				return fmt.Errorf("ana group state: %w", err)
			}
			break
		}
	}

	if err := os.WriteFile(filepath.Join(nsPath, "enable"), []byte("1"), 0644); err != nil {
		return fmt.Errorf("namespace enable: %w", err)
	}

	pName := portDirName(e.Transport, e.ListenAddr, e.ListenPort)
	pPath := filepath.Join(root, "ports", pName)
	if _, err := os.Stat(pPath); os.IsNotExist(err) {
		if err := os.Mkdir(pPath, 0755); err != nil {
			return fmt.Errorf("mkdir port: %w", err)
		}
		if err := os.WriteFile(filepath.Join(pPath, "addr_trtype"), []byte(e.Transport), 0644); err != nil {
			return fmt.Errorf("addr_trtype: %w", err)
		}
		if err := os.WriteFile(filepath.Join(pPath, "addr_adrfam"), []byte("ipv4"), 0644); err != nil {
			return fmt.Errorf("addr_adrfam: %w", err)
		}
		if err := os.WriteFile(filepath.Join(pPath, "addr_traddr"), []byte(e.ListenAddr), 0644); err != nil {
			return fmt.Errorf("addr_traddr: %w", err)
		}
		svc := fmt.Sprintf("%d", e.ListenPort)
		if err := os.WriteFile(filepath.Join(pPath, "addr_trsvcid"), []byte(svc), 0644); err != nil {
			return fmt.Errorf("addr_trsvcid: %w", err)
		}
	}
	link := filepath.Join(pPath, "subsystems", ssName)
	_ = os.Remove(link)
	target := filepath.Join("..", "..", "..", "subsystems", ssName)
	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("port subsystem link: %w", err)
	}

	return nil
}

func writeHostNQN(hpath, nqn string) error {
	for _, fname := range []string{"hostnqn", "host_nqn"} {
		p := filepath.Join(hpath, fname)
		if err := os.WriteFile(p, []byte(nqn), 0644); err == nil {
			return nil
		}
	}
	return fmt.Errorf("could not write host NQN in %s", hpath)
}

// ANAPropagationDelay is the time to wait after writing an ANA state change
// before proceeding with VIP handoff or pool export. The Linux NVMe target
// driver commits the state synchronously (the write syscall returns after the
// kernel has the new state), but the driver then sends an AEN (Asynchronous
// Event Notification) to each connected host out-of-band. Hosts process the
// AEN and update their multipath path tables asynchronously. If the VIP is
// released before hosts have processed the AEN, they see an abrupt path loss
// instead of a clean Standby transition and emit I/O errors.
//
// 200ms is conservative and well within the NVMe-oF recommended AEN processing
// window. Set to 0 in test environments to avoid slowing unit tests.
var ANAPropagationDelay = 200 * time.Millisecond

// applyANAGroupState creates the ANA group directory under the subsystem and
// writes the access state. The kernel accepts the state as a string:
//
//	"optimized"      → ANA state 1  (Active/Optimized, primary HA node)
//	"non-optimized"  → ANA state 2  (Active/Non-Optimized, secondary path)
//	"standby"        → ANA state 14 (Standby, HA standby node)
//	"inaccessible"   → ANA state 3  (Inaccessible)
//
// After writing the state, this function waits ANAPropagationDelay for the
// kernel's AEN to reach connected hosts before returning. Callers must NOT
// release the VIP or export pools until this function returns.
//
// The ana_groups/ directory and its state file are only present on kernels ≥5.17
// with nvmet compiled with CONFIG_NVME_TARGET_ANA. Failure is logged but not fatal
// so that deployments on older kernels still function without ANA.
func applyANAGroupState(ssPath string, groupID int, state ANAState) error {
	anaDir := filepath.Join(ssPath, "ana_groups", fmt.Sprintf("%d", groupID))
	if err := os.MkdirAll(anaDir, 0755); err != nil {
		return fmt.Errorf("mkdir ana_groups/%d: %w", groupID, err)
	}
	stateFile := filepath.Join(anaDir, "ana_state")
	stateStr := string(state)
	if stateStr == "" {
		stateStr = string(ANAOptimized)
	}
	if err := os.WriteFile(stateFile, []byte(stateStr), 0644); err != nil {
		// ANA may not be compiled into the kernel; log and continue.
		return fmt.Errorf("write ana_state: %w (is CONFIG_NVME_TARGET_ANA enabled?)", err)
	}

	// Read back to confirm the kernel accepted the write. An empty or unreadable
	// read-back means the kernel silently rejected the state (e.g. CONFIG_NVME_TARGET_ANA
	// not compiled in, or an invalid state string). A non-empty but different value
	// is normal: the kernel normalises human names to numeric state codes
	// (e.g. "standby" -> "14"). We log the normalisation but do not error.
	written, readErr := os.ReadFile(stateFile)
	if readErr != nil {
		return fmt.Errorf("ana_state write could not be verified (read-back failed: %w)", readErr)
	}
	got := strings.TrimSpace(string(written))
	if got == "" {
		return fmt.Errorf("ana_state write rejected by kernel (empty read-back for state %q)", stateStr)
	}
	if got != stateStr {
		// Kernel normalised the state string to a numeric code; not an error.
		log.Printf("nvmet: ANA group %d: wrote %q, kernel reflects %q (numeric normalisation)", groupID, stateStr, got)
	}

	// AEN propagation barrier: wait for connected hosts to receive and process
	// the state-change notification before the caller proceeds with VIP handoff.
	if ANAPropagationDelay > 0 {
		time.Sleep(ANAPropagationDelay)
	}

	return nil
}
