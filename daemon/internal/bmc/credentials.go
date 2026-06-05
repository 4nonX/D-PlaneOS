package bmc

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
)

// readPasswordFile reads the BMC password from the 0600 file on disk.
// The password is never stored in the database; only the file path is.
func readPasswordFile(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("no BMC password file configured")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read BMC password file %s: %w", path, err)
	}
	return strings.TrimSpace(string(raw)), nil
}

// LoadFromDB reads BMC credentials and the pinned TLS certificate from
// the ha_fencing_config table. Returns an error if fencing is not configured.
func LoadFromDB(db *sql.DB) (Credentials, PinnedCert, error) {
	var host, user, passFile, fingerprint string
	var pinnedAtNull sql.NullTime
	err := db.QueryRow(`
		SELECT COALESCE(bmc_ip,''), COALESCE(bmc_user,''),
		       COALESCE(bmc_password_file,''),
		       COALESCE(bmc_tls_fingerprint,''),
		       bmc_tls_pinned_at
		FROM ha_fencing_config WHERE id = 1
	`).Scan(&host, &user, &passFile, &fingerprint, &pinnedAtNull)
	if err == sql.ErrNoRows {
		return Credentials{}, PinnedCert{}, fmt.Errorf("no fencing config found: configure a BMC under Settings > HA > Fencing")
	}
	if err != nil {
		return Credentials{}, PinnedCert{}, fmt.Errorf("load BMC config: %w", err)
	}
	if host == "" {
		return Credentials{}, PinnedCert{}, fmt.Errorf("BMC IP not configured: add it under Settings > HA > Fencing")
	}

	pw, err := readPasswordFile(passFile)
	if err != nil {
		return Credentials{}, PinnedCert{}, err
	}

	creds := Credentials{
		Host:     host,
		Username: user,
		Password: pw,
	}
	var pin PinnedCert
	if fingerprint != "" {
		pin.Fingerprint = fingerprint
		if pinnedAtNull.Valid {
			pin.PinnedAt = pinnedAtNull.Time
		}
	}
	return creds, pin, nil
}

// SaveTLSFingerprint stores the TOFU-captured certificate fingerprint.
func SaveTLSFingerprint(db *sql.DB, pin PinnedCert) error {
	_, err := db.Exec(`
		UPDATE ha_fencing_config
		SET bmc_tls_fingerprint = $1, bmc_tls_pinned_at = NOW()
		WHERE id = 1`,
		pin.Fingerprint)
	return err
}

// ClearTLSFingerprint removes the stored fingerprint so the next connection
// re-enrolls. Requires the caller to have verified operator intent (AAL2).
func ClearTLSFingerprint(db *sql.DB) error {
	_, err := db.Exec(`
		UPDATE ha_fencing_config
		SET bmc_tls_fingerprint = '', bmc_tls_pinned_at = NULL
		WHERE id = 1`)
	return err
}
