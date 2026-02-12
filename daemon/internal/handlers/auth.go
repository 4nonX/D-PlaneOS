package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	db *sql.DB
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *sql.DB) *AuthHandler {
	return &AuthHandler{db: db}
}

// --- POST /api/auth/login ---

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": "Invalid request body",
		})
		return
	}

	// Allowlist validation
	if !isAlphanumericDash(req.Username) || len(req.Username) > 64 {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": "Invalid username format",
		})
		return
	}

	// Lookup user
	var userID int
	var storedHash string
	var active int
	err := h.db.QueryRow(
		`SELECT id, password_hash, active FROM users WHERE username = ? LIMIT 1`,
		req.Username,
	).Scan(&userID, &storedHash, &active)

	if err == sql.ErrNoRows {
		// Constant-time: still do a bcrypt compare to prevent timing attacks
		bcrypt.CompareHashAndPassword([]byte("$2a$10$dummyhashfortimingoracle000000000000000000000000000000"), []byte(req.Password))
		log.Printf("AUTH FAIL: unknown user %q from %s", req.Username, r.RemoteAddr)
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false, "error": "Invalid credentials",
		})
		return
	} else if err != nil {
		log.Printf("AUTH ERROR: db query failed: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false, "error": "Internal error",
		})
		return
	}

	if active != 1 {
		log.Printf("AUTH FAIL: disabled user %q from %s", req.Username, r.RemoteAddr)
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false, "error": "Account disabled",
		})
		return
	}

	// Verify password (bcrypt)
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.Password)); err != nil {
		log.Printf("AUTH FAIL: wrong password for %q from %s", req.Username, r.RemoteAddr)
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false, "error": "Invalid credentials",
		})
		return
	}

	// Generate session token (32 bytes = 64 hex chars)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Printf("AUTH ERROR: failed to generate session token: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false, "error": "Internal error",
		})
		return
	}
	sessionID := hex.EncodeToString(tokenBytes)

	// Session expires in 24 hours
	expiresAt := time.Now().Add(24 * time.Hour).Unix()

	// Insert session
	_, err = h.db.Exec(
		`INSERT INTO sessions (session_id, user_id, username, expires_at) VALUES (?, ?, ?, ?)`,
		sessionID, userID, req.Username, expiresAt,
	)
	if err != nil {
		log.Printf("AUTH ERROR: failed to create session: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false, "error": "Internal error",
		})
		return
	}

	// Audit log
	h.auditLog(req.Username, "login", "Session created", r.RemoteAddr)

	log.Printf("AUTH OK: %q from %s", req.Username, r.RemoteAddr)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"session_id": sessionID,
		"username":   req.Username,
		"expires_at": expiresAt,
	})
}

// --- POST /api/auth/logout ---

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get("X-Session-ID")
	username := r.Header.Get("X-User")

	if sessionID != "" {
		h.db.Exec(`DELETE FROM sessions WHERE session_id = ?`, sessionID)
		h.auditLog(username, "logout", "Session destroyed", r.RemoteAddr)
		log.Printf("LOGOUT: %q from %s", username, r.RemoteAddr)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// --- GET /api/auth/check ---

func (h *AuthHandler) Check(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get("X-Session-ID")

	if sessionID == "" {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"authenticated": false,
		})
		return
	}

	var username string
	var expiresAt int64
	err := h.db.QueryRow(
		`SELECT username, COALESCE(expires_at, 0) FROM sessions 
		 WHERE session_id = ? AND (expires_at IS NULL OR expires_at > ?)`,
		sessionID, time.Now().Unix(),
	).Scan(&username, &expiresAt)

	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"authenticated": false,
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": true,
		"user": map[string]interface{}{
			"username": username,
		},
	})
}

// --- GET /api/auth/session ---

func (h *AuthHandler) Session(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get("X-Session-ID")

	if sessionID == "" {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false, "error": "No session",
		})
		return
	}

	var username, email, role string
	var userID int
	err := h.db.QueryRow(
		`SELECT u.id, u.username, COALESCE(u.email,''), COALESCE(u.role,'user')
		 FROM sessions s JOIN users u ON s.username = u.username
		 WHERE s.session_id = ? AND (s.expires_at IS NULL OR s.expires_at > ?) AND u.active = 1`,
		sessionID, time.Now().Unix(),
	).Scan(&userID, &username, &email, &role)

	if err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false, "error": "Invalid session",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"user": map[string]interface{}{
			"id":       userID,
			"username": username,
			"email":    email,
			"role":     role,
		},
	})
}

// --- POST /api/auth/change-password ---

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false, "error": "Not authenticated",
		})
		return
	}

	// Get username from session
	var username string
	err := h.db.QueryRow(
		`SELECT username FROM sessions WHERE session_id = ? AND (expires_at IS NULL OR expires_at > ?)`,
		sessionID, time.Now().Unix(),
	).Scan(&username)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false, "error": "Invalid session",
		})
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": "Invalid request",
		})
		return
	}

	// Validate new password length
	if len(req.NewPassword) < 8 {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": "Password must be at least 8 characters",
		})
		return
	}

	// Verify current password
	var storedHash string
	err = h.db.QueryRow(`SELECT password_hash FROM users WHERE username = ?`, username).Scan(&storedHash)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false, "error": "Internal error",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.CurrentPassword)); err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false, "error": "Current password is incorrect",
		})
		return
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false, "error": "Internal error",
		})
		return
	}

	// Update
	_, err = h.db.Exec(`UPDATE users SET password_hash = ? WHERE username = ?`, string(newHash), username)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false, "error": "Failed to update password",
		})
		return
	}

	h.auditLog(username, "password_changed", "Password changed", r.RemoteAddr)
	log.Printf("PASSWORD CHANGED: %q from %s", username, r.RemoteAddr)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Password changed successfully",
	})
}

// --- GET /api/csrf ---

func (h *AuthHandler) CSRFToken(w http.ResponseWriter, r *http.Request) {
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"csrf_token": token,
	})
}

// --- Helpers ---

func (h *AuthHandler) auditLog(user, action, details, ip string) {
	h.db.Exec(
		`INSERT INTO audit_logs (user, action, details, ip_address) VALUES (?, ?, ?, ?)`,
		user, action, details, ip,
	)
}


func isAlphanumericDash(s string) bool {
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return len(s) > 0
}

// CleanExpiredSessions removes expired sessions (call periodically)
func (h *AuthHandler) CleanExpiredSessions() {
	result, err := h.db.Exec(`DELETE FROM sessions WHERE expires_at IS NOT NULL AND expires_at < ?`, time.Now().Unix())
	if err != nil {
		log.Printf("Session cleanup error: %v", err)
		return
	}
	if count, _ := result.RowsAffected(); count > 0 {
		log.Printf("Cleaned %d expired sessions", count)
	}
}

