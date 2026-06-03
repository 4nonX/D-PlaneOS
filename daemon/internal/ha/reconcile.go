package ha

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"dplaned/internal/security"
)

// SyncStatus is returned by GET /api/ha/sync/status and consumed by peers
// during startup reconciliation to detect stale (zombie) nodes.
type SyncStatus struct {
	IsActive bool             `json:"is_active"`
	Pools    map[string]int64 `json:"pools"` // pool name → ZFS TXG (transaction group ID)
}

// GetLocalSyncStatus builds a SyncStatus for this node by reading ZFS TXGs
// from all locally visible pools. Higher TXG = more recent data.
func GetLocalSyncStatus(isActive bool) SyncStatus {
	s := SyncStatus{
		IsActive: isActive,
		Pools:    make(map[string]int64),
	}
	out, err := exec.Command("zpool", "list", "-H", "-o", "name").Output()
	if err != nil {
		return s
	}
	for _, pool := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		pool = strings.TrimSpace(pool)
		if pool == "" {
			continue
		}
		s.Pools[pool] = localPoolTXG(pool)
	}
	return s
}

// localPoolTXG returns the latest committed ZFS transaction group for a pool.
// Returns 0 on error (pool not imported, non-existent, or invalid name).
func localPoolTXG(pool string) int64 {
	if err := security.ValidatePoolName(pool); err != nil {
		log.Printf("HA RECONCILE: localPoolTXG: %v", err)
		return 0
	}
	out, err := exec.Command("zfs", "get", "-H", "-p", "-o", "value", "txg", pool).Output()
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return n
}

// queryPeerSyncStatus fetches the SyncStatus from a peer daemon.
func queryPeerSyncStatus(peerAddr string) (SyncStatus, error) {
	var s SyncStatus
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(peerAddr + "/api/ha/sync/status")
	if err != nil {
		return s, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return s, fmt.Errorf("peer returned %d", resp.StatusCode)
	}
	err = json.NewDecoder(resp.Body).Decode(&s)
	return s, err
}

// StartupReconciliation is called once at daemon boot, before the heartbeat loop starts.
// It detects the "zombie resurrection" scenario: a node returning after a failover
// with a ZFS state that is days/weeks behind the currently-active peer.
//
// If a peer is active AND holds a newer TXG for the shared pool, this node enters
// Subordinate Mode: local pool is locked read-only and an async catch-up sync begins.
// Only after the sync completes does this node re-enable auto-failover as a valid Standby.
func (m *Manager) StartupReconciliation() {
	replCfg := m.GetReplicationConfig()
	if replCfg == nil {
		return
	}

	localTXG := localPoolTXG(replCfg.LocalPool)
	if localTXG == 0 {
		return
	}

	m.mu.RLock()
	peerAddrs := make([]string, 0, len(m.nodes))
	for _, n := range m.nodes {
		peerAddrs = append(peerAddrs, n.Address)
	}
	m.mu.RUnlock()

	for _, addr := range peerAddrs {
		status, err := queryPeerSyncStatus(addr)
		if err != nil || !status.IsActive {
			continue
		}

		peerTXG := status.Pools[replCfg.RemotePool]
		if peerTXG == 0 {
			peerTXG = status.Pools[replCfg.LocalPool]
		}
		if peerTXG <= localTXG {
			continue
		}

		delta := peerTXG - localTXG
		log.Printf("HA RECONCILE: Zombie boot detected - peer at %s is active (TXG %d vs local %d, Δ%d). Entering Subordinate Mode to prevent stale data serving.",
			addr, peerTXG, localTXG, delta)

		if err := ValidateReplicationConfig(replCfg); err != nil {
			log.Printf("HA RECONCILE: aborting subordinate mode - invalid replication config: %v", err)
			return
		}

		m.mu.Lock()
		m.subordinateMode = true
		m.mu.Unlock()
		go m.persistClusterState()

		exec.Command("zfs", "set", "readonly=on", replCfg.LocalPool).Run() //nolint
		log.Printf("HA RECONCILE: Pool %s locked read-only. Starting catch-up sync from active peer...", replCfg.LocalPool)

		go func(cfg *ReplicationConfig) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
			defer cancel()

			if err := m.catchUpFromPeer(ctx, cfg); err != nil {
				log.Printf("HA RECONCILE: Catch-up sync failed: %v. Node remains in Subordinate Mode - manual intervention required (zfs recv or /api/ha/clear_fault).", err)
				return
			}

			exec.Command("zfs", "set", "readonly=off", cfg.LocalPool).Run() //nolint
			m.mu.Lock()
			m.subordinateMode = false
			m.mu.Unlock()
			go m.persistClusterState()
			log.Printf("HA RECONCILE: Catch-up complete. Pool %s unlocked. Node is now a valid Standby.", cfg.LocalPool)
		}(replCfg)

		return
	}
}

// catchUpFromPeer performs a reverse-direction ZFS receive: connect to the
// active peer via native SSH, stream its latest snapshot, receive into the
// local pool. No subprocess shell is spawned for SSH; no piped remote commands
// are used. All data received from the remote is validated before use in any
// subsequent exec call.
func (m *Manager) catchUpFromPeer(ctx context.Context, cfg *ReplicationConfig) error {
	if err := ValidateReplicationConfig(cfg); err != nil {
		return err
	}
	log.Printf("HA RECONCILE: Receiving catch-up stream from %s@%s (%s → %s)",
		cfg.RemoteUser, cfg.RemoteHost, cfg.RemotePool, cfg.LocalPool)

	client, err := openSSHClient(cfg)
	if err != nil {
		return fmt.Errorf("SSH connect to %s: %w", cfg.RemoteHost, err)
	}
	defer client.Close()

	// List remote snapshots without a shell pipe. remoteOutput runs via the SSH
	// exec channel with each argument single-quote-wrapped; the last line is
	// extracted in Go, not via "| tail -n 1" on the remote shell.
	remoteListOut, remoteListErr := remoteOutput(client,
		"zfs", "list", "-t", "snapshot", "-H", "-o", "name", "-s", "creation", "-r", cfg.RemotePool,
	)
	if remoteListErr != nil {
		return fmt.Errorf("list remote snapshots: %w", remoteListErr)
	}
	latestRemoteSnap := lastNonEmptyLine(string(remoteListOut))
	if latestRemoteSnap == "" {
		return fmt.Errorf("no snapshots on remote pool %s - cannot catch up", cfg.RemotePool)
	}
	// Validate snapshot name received from the remote before using it in any
	// subsequent exec call or SSH command string.
	if err := security.ValidateSnapshotName(latestRemoteSnap); err != nil {
		return fmt.Errorf("remote returned invalid snapshot name %q: %w", latestRemoteSnap, err)
	}

	// Find our latest local snapshot to use as incremental base (avoids full send).
	localListOut, _ := exec.CommandContext(ctx,
		"zfs", "list", "-t", "snapshot", "-H", "-o", "name", "-s", "creation", "-r", cfg.LocalPool,
	).Output()
	var baseSnap string
	if localLatest := lastNonEmptyLine(string(localListOut)); localLatest != "" {
		parts := strings.SplitN(localLatest, "@", 2)
		if len(parts) == 2 {
			// Translate local snapshot name to remote pool context for the
			// incremental base argument sent to the remote zfs send.
			candidate := cfg.RemotePool + "@" + parts[1]
			if err := security.ValidateSnapshotName(candidate); err == nil {
				baseSnap = candidate
			}
		}
	}

	// Build the remote zfs send command as a slice of separate arguments.
	// shellQuoteArgs wraps each in single quotes so the remote shell treats
	// every argument as a single opaque token - no string formatting injection.
	var sendArgs []string
	if baseSnap != "" {
		sendArgs = []string{"zfs", "send", "-R", "-i", baseSnap, latestRemoteSnap}
	} else {
		sendArgs = []string{"zfs", "send", "-R", latestRemoteSnap}
	}

	// Open an SSH session for the remote sender, pipe its stdout to local recv.
	sendSession, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new SSH session for send: %w", err)
	}
	defer sendSession.Close()

	var sendStderr bytes.Buffer
	sendSession.Stderr = &sendStderr

	// Local zfs recv subprocess reads from the SSH session's stdout.
	receiver := exec.CommandContext(ctx, "zfs", "recv", "-F", "-s", cfg.LocalPool)

	senderStdout, err := sendSession.StdoutPipe()
	if err != nil {
		return fmt.Errorf("SSH session stdout pipe: %w", err)
	}
	receiver.Stdin = senderStdout

	if err := sendSession.Start(shellQuoteArgs(sendArgs)); err != nil {
		return fmt.Errorf("start remote zfs send: %w", err)
	}
	if err := receiver.Start(); err != nil {
		sendSession.Signal(ssh.SIGTERM) //nolint
		sendSession.Wait()              //nolint
		return fmt.Errorf("start local zfs recv: %w", err)
	}

	senderErr := sendSession.Wait()
	recvErr := receiver.Wait()

	if senderErr != nil {
		var sshExitErr *ssh.ExitError
		if errors.As(senderErr, &sshExitErr) {
			if errOut := strings.TrimSpace(sendStderr.String()); errOut != "" {
				return fmt.Errorf("remote zfs send failed (exit %d): %s", sshExitErr.ExitStatus(), errOut)
			}
		}
		return fmt.Errorf("remote zfs send: %w", senderErr)
	}
	if recvErr != nil {
		return fmt.Errorf("local zfs recv: %w", recvErr)
	}
	return nil
}
