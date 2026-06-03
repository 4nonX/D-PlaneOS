package ha

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const haKnownHostsPath = "/var/lib/dplaneos/ha_known_hosts"

// knownHostsMu serialises reads and writes to haKnownHostsPath so two
// concurrent first-connections cannot both decide to add the same host.
var knownHostsMu sync.Mutex

// openSSHClient creates a native SSH client for the given replication config.
// Authentication uses the key file; host keys are verified against
// haKnownHostsPath with accept-new semantics (first connection stores the key,
// subsequent connections verify it). This replaces all exec.Command("ssh", ...)
// calls and eliminates the subprocess shell that was the injection surface.
func openSSHClient(cfg *ReplicationConfig) (*ssh.Client, error) {
	keyBytes, err := os.ReadFile(cfg.SSHKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read SSH key %s: %w", cfg.SSHKeyPath, err)
	}
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse SSH private key: %w", err)
	}
	hkCB, err := acceptNewHostKeyCallback(haKnownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("host key callback: %w", err)
	}
	config := &ssh.ClientConfig{
		User:            cfg.RemoteUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hkCB,
		Timeout:         15 * time.Second,
	}
	addr := net.JoinHostPort(cfg.RemoteHost, fmt.Sprintf("%d", cfg.RemotePort))
	return ssh.Dial("tcp", addr, config)
}

// remoteOutput runs a command on the remote host via the SSH exec channel and
// returns stdout. Each argument is single-quote-wrapped before being joined,
// so the remote shell treats each as a single opaque token with no glob
// expansion, variable substitution, or word splitting.
//
// All callers must pre-validate every argument through the security allowlist.
// Do not pass shell pipeline syntax (|, ;, &&) as arguments.
func remoteOutput(client *ssh.Client, args ...string) ([]byte, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new SSH session: %w", err)
	}
	defer session.Close()
	return session.Output(shellQuoteArgs(args))
}

// shellQuoteArgs joins args into a shell command string where each argument is
// wrapped in single quotes. Safe for all inputs validated to contain only
// [a-zA-Z0-9_\-\./@:] - none of which have special meaning inside single quotes.
func shellQuoteArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = "'" + a + "'"
	}
	return strings.Join(quoted, " ")
}

// lastNonEmptyLine returns the last non-empty line of s, replacing the shell
// pipeline pattern "cmd | tail -n 1" with in-process Go string handling.
func lastNonEmptyLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if trimmed := strings.TrimSpace(lines[i]); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// acceptNewHostKeyCallback builds an SSH HostKeyCallback that verifies known
// hosts and adds unknown ones on first connection (accept-new semantics,
// equivalent to StrictHostKeyChecking=accept-new in ssh(1)). The known_hosts
// file is created at haKnownHostsPath if absent.
//
// The file format is one entry per line: "hostname keytype base64key"
// (the keytype+base64key portion is compatible with authorized_keys format so
// ssh.ParseAuthorizedKey can parse it directly). This format is implemented
// using only the vendored golang.org/x/crypto/ssh primitives, without requiring
// the knownhosts sub-package which is not in the vendor tree.
func acceptNewHostKeyCallback(path string) (ssh.HostKeyCallback, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create known_hosts dir: %w", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, nil, 0600); err != nil {
			return nil, fmt.Errorf("create known_hosts file: %w", err)
		}
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		knownHostsMu.Lock()
		defer knownHostsMu.Unlock()
		return checkOrAddHost(path, hostname, key)
	}, nil
}

// checkOrAddHost verifies the presented key against the stored entry for
// hostname. If no entry exists, the key is recorded (accept-new). If an entry
// exists with a different fingerprint, the connection is rejected.
// Must be called with knownHostsMu held.
func checkOrAddHost(path, hostname string, key ssh.PublicKey) error {
	presentedFP := ssh.FingerprintSHA256(key)

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read known_hosts %s: %w", path, err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Format: "hostname keytype base64key"
		idx := strings.Index(line, " ")
		if idx < 0 {
			continue
		}
		if line[:idx] != hostname {
			continue
		}
		// Parse the "keytype base64key" portion using the ssh package directly.
		stored, _, _, _, parseErr := ssh.ParseAuthorizedKey([]byte(line[idx+1:]))
		if parseErr != nil {
			continue
		}
		storedFP := ssh.FingerprintSHA256(stored)
		if storedFP != presentedFP {
			return fmt.Errorf(
				"SSH host key mismatch for %s: stored %s, presented %s - possible MITM attack",
				hostname, storedFP, presentedFP,
			)
		}
		return nil // Known and verified.
	}

	// Host not yet in file - record it (accept-new semantics).
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("open known_hosts for write: %w", err)
	}
	defer f.Close()
	// ssh.MarshalAuthorizedKey produces "keytype base64key\n"; trim the newline
	// so we can prepend the hostname ourselves.
	keyPart := bytes.TrimRight(ssh.MarshalAuthorizedKey(key), "\n")
	_, err = fmt.Fprintf(f, "%s %s\n", hostname, keyPart)
	return err
}
