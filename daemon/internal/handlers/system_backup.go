package handlers

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
)

type SystemBackupHandler struct {
	dsn string
}

func NewSystemBackupHandler(dsn string) *SystemBackupHandler {
	return &SystemBackupHandler{dsn: dsn}
}

// DownloadBackup streams a pg_dump of the control-plane database as an attachment.
// GET /api/system/db/backup
func (h *SystemBackupHandler) DownloadBackup(w http.ResponseWriter, r *http.Request) {
	filename := fmt.Sprintf("dplaneos-backup-%s.dump", time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	var stderr bytes.Buffer
	cmd := exec.CommandContext(r.Context(), "pg_dump", "--format=custom", "--dbname="+h.dsn)
	cmd.Stdout = w
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Printf("DB BACKUP: pg_dump failed: %v: %s", err, stderr.String())
	}
}

// RestoreBackup accepts a pg_dump file upload and restores the database.
// POST /api/system/db/restore
func (h *SystemBackupHandler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 512<<20)
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		respondErrorSimple(w, "Failed to parse upload: "+err.Error(), http.StatusBadRequest)
		return
	}
	f, _, err := r.FormFile("backup")
	if err != nil {
		respondErrorSimple(w, "Missing 'backup' file field in form", http.StatusBadRequest)
		return
	}
	defer f.Close()

	tmp, err := os.CreateTemp("", "dplaneos-restore-*.dump")
	if err != nil {
		respondErrorSimple(w, "Failed to create temp file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp.Name())

	if _, err := io.Copy(tmp, f); err != nil {
		tmp.Close()
		respondErrorSimple(w, "Failed to receive upload", http.StatusInternalServerError)
		return
	}
	tmp.Close()

	out, err := exec.Command("pg_restore", "--clean", "--if-exists", "--dbname="+h.dsn, tmp.Name()).CombinedOutput()
	if err != nil {
		log.Printf("DB RESTORE: pg_restore failed: %v: %s", err, out)
		respondErrorSimple(w, "Restore failed: "+string(out), http.StatusInternalServerError)
		return
	}

	respondOK(w, map[string]interface{}{"success": true, "message": "Database restored successfully"})
}
