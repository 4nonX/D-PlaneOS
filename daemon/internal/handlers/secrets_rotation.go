package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"dplaned/internal/secrets"
)

type SecretsRotationHandler struct {
	db      *sql.DB
	keyPath string
}

func NewSecretsRotationHandler(db *sql.DB, keyPath string) *SecretsRotationHandler {
	return &SecretsRotationHandler{db: db, keyPath: keyPath}
}

// reenc decrypts with openOld, re-encrypts with sealNew.
// Empty input is returned as-is (no-op for unset optional fields).
func reenc(openOld, sealNew func(string) (string, error), s string) (string, error) {
	if s == "" {
		return "", nil
	}
	plain, err := openOld(s)
	if err != nil {
		return "", err
	}
	return sealNew(plain)
}

// RotateKeys re-encrypts every stored secret under a new AES-256-GCM key.
// POST /api/system/secrets/rotate
func (h *SecretsRotationHandler) RotateKeys(w http.ResponseWriter, r *http.Request) {
	openOld, sealNew, commit, err := secrets.PrepareRotation(h.keyPath)
	if err != nil {
		respondErrorSimple(w, "Failed to prepare rotation: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		respondErrorSimple(w, "Failed to begin transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	rotated := 0

	// --- telegram_config.bot_token ---
	var sealedToken string
	if err := tx.QueryRow("SELECT COALESCE(bot_token,'') FROM telegram_config WHERE id=1").Scan(&sealedToken); err == nil && sealedToken != "" {
		newSealed, err := reenc(openOld, sealNew, sealedToken)
		if err != nil {
			log.Printf("ROTATE: telegram bot_token: %v", err)
			respondErrorSimple(w, "Failed to re-encrypt telegram token", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec("UPDATE telegram_config SET bot_token=$1 WHERE id=1", newSealed); err != nil {
			respondErrorSimple(w, "Failed to update telegram token", http.StatusInternalServerError)
			return
		}
		rotated++
	}

	// --- ldap_config.bind_password ---
	var ldapPwd string
	if err := tx.QueryRow("SELECT COALESCE(bind_password,'') FROM ldap_config WHERE id=1").Scan(&ldapPwd); err == nil && ldapPwd != "" {
		newSealed, err := reenc(openOld, sealNew, ldapPwd)
		if err != nil {
			log.Printf("ROTATE: ldap bind_password: %v", err)
			respondErrorSimple(w, "Failed to re-encrypt LDAP password", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec("UPDATE ldap_config SET bind_password=$1 WHERE id=1", newSealed); err != nil {
			respondErrorSimple(w, "Failed to update LDAP password", http.StatusInternalServerError)
			return
		}
		rotated++
	}

	// --- ad_domains.bind_password (one row per domain) ---
	adRows, err := tx.Query("SELECT id, COALESCE(bind_password,'') FROM ad_domains")
	if err != nil {
		respondErrorSimple(w, "Failed to query AD domains", http.StatusInternalServerError)
		return
	}
	type adRow struct {
		id  int64
		pwd string
	}
	var adDomains []adRow
	for adRows.Next() {
		var a adRow
		if err := adRows.Scan(&a.id, &a.pwd); err == nil && a.pwd != "" {
			adDomains = append(adDomains, a)
		}
	}
	adRows.Close()
	for _, a := range adDomains {
		newSealed, err := reenc(openOld, sealNew, a.pwd)
		if err != nil {
			log.Printf("ROTATE: ad_domains id=%d: %v", a.id, err)
			respondErrorSimple(w, "Failed to re-encrypt AD domain password", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec("UPDATE ad_domains SET bind_password=$1 WHERE id=$2", newSealed, a.id); err != nil {
			respondErrorSimple(w, "Failed to update AD domain password", http.StatusInternalServerError)
			return
		}
		rotated++
	}

	// --- git_credentials.token + ssh_key ---
	gcRows, err := tx.Query("SELECT id, COALESCE(token,''), COALESCE(ssh_key,'') FROM git_credentials")
	if err != nil {
		respondErrorSimple(w, "Failed to query git credentials", http.StatusInternalServerError)
		return
	}
	type gcRow struct {
		id     int64
		token  string
		sshKey string
	}
	var gcreds []gcRow
	for gcRows.Next() {
		var g gcRow
		if err := gcRows.Scan(&g.id, &g.token, &g.sshKey); err == nil {
			if g.token != "" || g.sshKey != "" {
				gcreds = append(gcreds, g)
			}
		}
	}
	gcRows.Close()
	for _, g := range gcreds {
		newToken, err := reenc(openOld, sealNew, g.token)
		if err != nil {
			log.Printf("ROTATE: git_credentials id=%d token: %v", g.id, err)
			respondErrorSimple(w, "Failed to re-encrypt git token", http.StatusInternalServerError)
			return
		}
		newSSH, err := reenc(openOld, sealNew, g.sshKey)
		if err != nil {
			log.Printf("ROTATE: git_credentials id=%d ssh_key: %v", g.id, err)
			respondErrorSimple(w, "Failed to re-encrypt git SSH key", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec("UPDATE git_credentials SET token=$1, ssh_key=$2 WHERE id=$3", newToken, newSSH, g.id); err != nil {
			respondErrorSimple(w, "Failed to update git credentials", http.StatusInternalServerError)
			return
		}
		rotated++
	}

	// --- oidc_config.client_secret ---
	var oidcSecret string
	if err := tx.QueryRow("SELECT COALESCE(client_secret,'') FROM oidc_config WHERE id=1").Scan(&oidcSecret); err == nil && oidcSecret != "" {
		newSealed, err := reenc(openOld, sealNew, oidcSecret)
		if err != nil {
			log.Printf("ROTATE: oidc client_secret: %v", err)
			respondErrorSimple(w, "Failed to re-encrypt OIDC secret", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec("UPDATE oidc_config SET client_secret=$1 WHERE id=1", newSealed); err != nil {
			respondErrorSimple(w, "Failed to update OIDC secret", http.StatusInternalServerError)
			return
		}
		rotated++
	}

	// --- totp_secrets.secret ---
	totpRows, err := tx.Query("SELECT user_id, secret FROM totp_secrets WHERE secret != ''")
	if err != nil {
		respondErrorSimple(w, "Failed to query TOTP secrets", http.StatusInternalServerError)
		return
	}
	type totpRow struct {
		userID int64
		secret string
	}
	var totpSecrets []totpRow
	for totpRows.Next() {
		var t totpRow
		if err := totpRows.Scan(&t.userID, &t.secret); err == nil {
			totpSecrets = append(totpSecrets, t)
		}
	}
	totpRows.Close()
	for _, t := range totpSecrets {
		newSealed, err := reenc(openOld, sealNew, t.secret)
		if err != nil {
			log.Printf("ROTATE: totp_secrets user_id=%d: %v", t.userID, err)
			respondErrorSimple(w, "Failed to re-encrypt TOTP secret", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec("UPDATE totp_secrets SET secret=$1 WHERE user_id=$2", newSealed, t.userID); err != nil {
			respondErrorSimple(w, "Failed to update TOTP secret", http.StatusInternalServerError)
			return
		}
		rotated++
	}

	// --- settings smtp_config.password (JSON field) ---
	var smtpValue string
	if err := tx.QueryRow("SELECT COALESCE(value,'') FROM settings WHERE key='smtp_config'").Scan(&smtpValue); err == nil && smtpValue != "" {
		var cfg smtpConfigForRotation
		if err := json.Unmarshal([]byte(smtpValue), &cfg); err == nil && cfg.Password != "" {
			newSealed, err := reenc(openOld, sealNew, cfg.Password)
			if err != nil {
				log.Printf("ROTATE: smtp_config.password: %v", err)
				respondErrorSimple(w, "Failed to re-encrypt SMTP password", http.StatusInternalServerError)
				return
			}
			cfg.Password = newSealed
			updated, err := json.Marshal(cfg)
			if err != nil {
				respondErrorSimple(w, "Failed to encode SMTP config", http.StatusInternalServerError)
				return
			}
			if _, err := tx.Exec("UPDATE settings SET value=$1, updated_at=NOW() WHERE key='smtp_config'", string(updated)); err != nil {
				respondErrorSimple(w, "Failed to update SMTP config", http.StatusInternalServerError)
				return
			}
			rotated++
		}
	}

	if err := tx.Commit(); err != nil {
		respondErrorSimple(w, "Failed to commit re-encrypted secrets", http.StatusInternalServerError)
		return
	}

	if err := commit(); err != nil {
		log.Printf("ROTATE: key file commit failed after DB commit: %v", err)
		respondErrorSimple(w, "Secrets re-encrypted but key file write failed - restart daemon immediately: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("SECRETS ROTATE: rotated %d secret values under new key", rotated)
	respondOK(w, map[string]any{"success": true, "rotated_count": rotated})
}

// smtpConfigForRotation mirrors SMTPConfig without pulling in the full type.
type smtpConfigForRotation struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	To       string `json:"to"`
	TLS      bool   `json:"tls"`
	Enabled  bool   `json:"enabled,omitempty"`
}
