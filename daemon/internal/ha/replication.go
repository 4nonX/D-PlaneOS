package ha

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"dplaned/internal/security"
	"dplaned/internal/zfs"
)

// ReplicationConfig holds the active-to-standby ZFS sync parameters.
type ReplicationConfig struct {
	LocalPool    string `json:"local_pool"`
	RemotePool   string `json:"remote_pool"`
	RemoteHost   string `json:"remote_host"`
	RemoteUser   string `json:"remote_user"`
	RemotePort   int    `json:"remote_port"`
	SSHKeyPath   string `json:"ssh_key_path"`
	IntervalSecs int    `json:"interval_secs"`
}

// ValidateReplicationConfig validates all fields of a ReplicationConfig before
// any value is used in an exec.Command or SSH remote command string. This closes
// the injection vector that exists when DB-sourced fields bypass the allowlist.
// Exported so the HTTP handler can validate at save time and return a clear error
// instead of letting bad values fail silently at replication run time.
func ValidateReplicationConfig(cfg *ReplicationConfig) error {
	if err := security.ValidatePoolName(cfg.LocalPool); err != nil {
		return fmt.Errorf("replication config: invalid local_pool: %w", err)
	}
	if err := security.ValidatePoolName(cfg.RemotePool); err != nil {
		return fmt.Errorf("replication config: invalid remote_pool: %w", err)
	}
	if err := security.ValidateHostname(cfg.RemoteHost); err != nil {
		return fmt.Errorf("replication config: invalid remote_host: %w", err)
	}
	if err := security.ValidateUnixUsername(cfg.RemoteUser); err != nil {
		return fmt.Errorf("replication config: invalid remote_user: %w", err)
	}
	if err := security.ValidateAbsolutePath(cfg.SSHKeyPath); err != nil {
		return fmt.Errorf("replication config: invalid ssh_key_path: %w", err)
	}
	if cfg.RemotePort < 1 || cfg.RemotePort > 65535 {
		return fmt.Errorf("replication config: invalid remote_port: %d", cfg.RemotePort)
	}
	return nil
}

// startReplicationLoop begins continuous ZFS sync to the standby peer
// if this node is acting as the primary.
func (m *Manager) startReplicationLoop(ctx context.Context, cfg *ReplicationConfig) {
	interval := time.Duration(cfg.IntervalSecs) * time.Second
	if interval < 10*time.Second {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("HA: continuous replication loop started (%s -> %s@%s:%s)",
		cfg.LocalPool, cfg.RemoteUser, cfg.RemoteHost, cfg.RemotePool)

	for {
		select {
		case <-ctx.Done():
			log.Printf("HA: continuous replication loop stopped")
			return
		case <-ticker.C:
			if m.Status().LocalNode.Role != RoleActive {
				continue
			}
			if !m.IsPatroniPrimary() {
				continue
			}
			if err := m.syncZFS(ctx, cfg); err != nil {
				log.Printf("HA Replication Error: %v", err)
			}
		}
	}
}

// syncZFS executes an incremental zfs send/recv using the native SSH library.
// No subprocess shell is spawned for SSH; no piped remote commands are used.
// All data received from the remote is validated before use in any exec call.
func (m *Manager) syncZFS(ctx context.Context, cfg *ReplicationConfig) error {
	if err := ValidateReplicationConfig(cfg); err != nil {
		return err
	}

	// List local snapshots directly via local exec (already safe - no user input in args).
	localOut, err := exec.CommandContext(ctx,
		"zfs", "list", "-t", "snapshot", "-o", "name", "-H", "-s", "creation", "-r", cfg.LocalPool,
	).Output()
	if err != nil {
		return fmt.Errorf("list local snapshots: %w", err)
	}
	latestLocalSnap := lastNonEmptyLine(string(localOut))
	if latestLocalSnap == "" {
		return nil // Nothing local to replicate.
	}

	// Open a single native SSH connection for all remote operations in this sync cycle.
	client, err := openSSHClient(cfg)
	if err != nil {
		return fmt.Errorf("SSH connect to %s: %w", cfg.RemoteHost, err)
	}
	defer client.Close()

	// List remote snapshots without a shell pipe: run the full list and take the
	// last line in Go. remoteOutput runs the command through the SSH exec channel
	// with each argument single-quote-wrapped, so no shell metacharacter is possible.
	remoteListOut, remoteListErr := remoteOutput(client,
		"zfs", "list", "-t", "snapshot", "-o", "name", "-H", "-s", "creation", "-r", cfg.RemotePool,
	)

	var latestRemoteSnapFull string
	if remoteListErr != nil {
		var sshExitErr *ssh.ExitError
		if errors.As(remoteListErr, &sshExitErr) && sshExitErr.ExitStatus() == 1 {
			// Exit 1 from zfs list means no snapshots exist on the remote pool.
			// Fall through with latestRemoteSnapFull == "" to trigger a full send.
		} else {
			return fmt.Errorf("list remote snapshots: %w", remoteListErr)
		}
	} else {
		latestRemoteSnapFull = lastNonEmptyLine(string(remoteListOut))
	}

	if latestRemoteSnapFull == "" {
		// Remote has no snapshots - full send.
		return m.executeSendRecv(ctx, cfg, client, "", latestLocalSnap)
	}

	// Validate the snapshot name we received from the remote before using it in
	// any subsequent exec call. This is the critical check that closes the
	// "validated input + string formatting" injection path.
	if err := security.ValidateSnapshotName(latestRemoteSnapFull); err != nil {
		return fmt.Errorf("remote returned invalid snapshot name %q: %w", latestRemoteSnapFull, err)
	}

	// Translate remote snapshot to our local pool context for existence check.
	// e.g. "remotetank/data@snap" -> "localtank/data@snap"
	localEquivalent := strings.Replace(latestRemoteSnapFull, cfg.RemotePool, cfg.LocalPool, 1)
	if latestLocalSnap == localEquivalent {
		return nil // Already in sync.
	}

	return m.executeSendRecv(ctx, cfg, client, localEquivalent, latestLocalSnap)
}

// executeSendRecv pipes a local zfs send stream to the remote via a native SSH
// session running zfs recv. No subprocess shell is used for the SSH side; the
// SSH exec channel carries the binary stream directly.
//
// baseSnap is the incremental base (empty string for a full send).
// targetSnap is the snapshot to send.
// client is an already-authenticated SSH connection to the remote host.
func (m *Manager) executeSendRecv(ctx context.Context, cfg *ReplicationConfig, client *ssh.Client, baseSnap, targetSnap string) error {
	if err := ValidateReplicationConfig(cfg); err != nil {
		return err
	}
	// Validate snapshot name arguments if provided.
	if baseSnap != "" {
		if err := security.ValidateSnapshotName(baseSnap); err != nil {
			return fmt.Errorf("invalid base snapshot %q: %w", baseSnap, err)
		}
	}
	if err := security.ValidateSnapshotName(targetSnap); err != nil {
		return fmt.Errorf("invalid target snapshot %q: %w", targetSnap, err)
	}

	// Local zfs send subprocess.
	var senderArgs []string
	if baseSnap != "" {
		senderArgs = []string{"send", "-P", "-R", "-i", baseSnap, targetSnap}
	} else {
		senderArgs = []string{"send", "-P", "-R", targetSnap}
	}
	sender := exec.CommandContext(ctx, "zfs", senderArgs...)

	senderStdout, err := sender.StdoutPipe()
	if err != nil {
		return fmt.Errorf("sender stdout pipe: %w", err)
	}
	senderStderr, err := sender.StderrPipe()
	if err != nil {
		return fmt.Errorf("sender stderr pipe: %w", err)
	}

	// Remote zfs recv via native SSH session (no subprocess shell for SSH).
	recvSession, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new SSH session for recv: %w", err)
	}
	defer recvSession.Close()

	recvSession.Stdin = senderStdout
	var recvStderr bytes.Buffer
	recvSession.Stderr = &recvStderr

	// Start remote receiver before local sender so recv is ready when data arrives.
	recvCmd := shellQuoteArgs([]string{"zfs", "recv", "-s", "-F", cfg.RemotePool})
	if err := recvSession.Start(recvCmd); err != nil {
		return fmt.Errorf("start remote zfs recv: %w", err)
	}

	if err := sender.Start(); err != nil {
		recvSession.Signal(ssh.SIGTERM) //nolint
		recvSession.Wait()              //nolint
		return fmt.Errorf("start local zfs send: %w", err)
	}

	// Parse send progress from sender's stderr and broadcast via WebSocket.
	var st zfs.SendProgressState
	go func() {
		sc := bufio.NewScanner(senderStderr)
		for sc.Scan() {
			line := sc.Text()
			if up, ok := zfs.FeedSendProgressLine(line, &st, 500*time.Millisecond); ok {
				up["source"] = "ha_zfs_sync"
				up["local_pool"] = cfg.LocalPool
				up["remote_pool"] = cfg.RemotePool
				up["remote_host"] = cfg.RemoteHost
				m.reportReplicationProgress(up)
			}
		}
	}()

	senderErr := sender.Wait()
	recvErr := recvSession.Wait()

	if senderErr != nil {
		return fmt.Errorf("zfs send failed: %w", senderErr)
	}
	if recvErr != nil {
		if errOut := strings.TrimSpace(recvStderr.String()); errOut != "" {
			return fmt.Errorf("remote zfs recv failed: %w (stderr: %s)", recvErr, errOut)
		}
		return fmt.Errorf("remote zfs recv failed: %w", recvErr)
	}
	return nil
}
