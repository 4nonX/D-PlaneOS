package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"strings"
	"time"

	"dplaned/internal/audit"
	"dplaned/internal/middleware"
	"dplaned/internal/alerts"
	"dplaned/internal/handlers"
	"dplaned/internal/monitoring"
	"dplaned/internal/security"
	"dplaned/internal/websocket"
	"dplaned/internal/zfs"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

const (
	Version = "2.1.0"
)

func main() {
	// Parse flags
	listenAddr := flag.String("listen", "127.0.0.1:9000", "Listen address")
	dbPath := flag.String("db", "/var/lib/dplaneos/dplaneos.db", "Path to SQLite database")
	telegramBot := flag.String("telegram-bot", "", "Telegram bot token (optional, for alerts)")
	telegramChat := flag.String("telegram-chat", "", "Telegram chat ID (optional, for alerts)")
	backupPath := flag.String("backup-path", "", "External path for DB backup (e.g., /mnt/usb/dplaneos-backup.db). If empty, backs up next to main DB.")
	configDir := flag.String("config-dir", "/etc/dplaneos", "Config directory (for NixOS: /var/lib/dplaneos/config)")
	smbConfPath := flag.String("smb-conf", "/etc/samba/smb.conf", "Path to write SMB config (for NixOS: /var/lib/dplaneos/smb-shares.conf)")
	flag.Parse()

	// Set configurable paths for NixOS compatibility
	handlers.SetConfigDir(*configDir)

	// Open database for buffered audit logging
	// Critical for systems with high I/O:
	// - WAL mode: concurrent reads during writes
	// - busy_timeout: wait 30s during WAL checkpoints (prevents "database locked")
	// - cache_size: 64MB in-memory cache
	// - wal_autocheckpoint: checkpoint every 1000 pages (~4MB) to prevent WAL bloat
	db, err := sql.Open("sqlite3", *dbPath+"?_journal_mode=WAL&_busy_timeout=30000&cache=shared&_cache_size=-65536&_wal_autocheckpoint=1000&_synchronous=FULL")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Force WAL checkpoint on startup to clean any leftover WAL from crashes
	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		log.Printf("Warning: initial WAL checkpoint failed: %v", err)
	}

	// Initialize database schema (IF NOT EXISTS — safe on every startup)
	if err := initSchema(db); err != nil {
		log.Fatalf("Database schema initialization failed: %v", err)
	}

	// Periodic WAL checkpoint every 5 minutes — safety net against WAL bloat
	// on systems with high audit logging rates (e.g., runaway container producing errors)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if _, err := db.Exec("PRAGMA wal_checkpoint(PASSIVE)"); err != nil {
				log.Printf("Warning: periodic WAL checkpoint failed: %v", err)
			}
		}
	}()

	// Daily VACUUM INTO — creates a clean backup copy of the database
	// Protects metadata against WAL corruption from hard power loss
	// Use -backup-path for off-pool backup (USB, second disk, NFS mount)
	go func() {
		dbBackupDest := *backupPath
		if dbBackupDest == "" {
			dbBackupDest = *dbPath + ".backup"
		}

		// Backup immediately on startup
		if _, err := db.Exec("VACUUM INTO ?", dbBackupDest); err != nil {
			log.Printf("Warning: startup DB backup failed: %v", err)
		} else {
			log.Printf("Startup DB backup created: %s", dbBackupDest)
		}

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if _, err := db.Exec("VACUUM INTO ?", dbBackupDest); err != nil {
				log.Printf("Warning: daily DB backup failed: %v", err)
			} else {
				log.Printf("Daily DB backup created: %s", dbBackupDest)
			}
		}
	}()

	// Initialize buffered audit logging (non-blocking)
	bufferedLogger := audit.NewBufferedLogger(db, 100, 5*time.Second)
	bufferedLogger.Start()
	defer bufferedLogger.Stop()

	// Initialize database connection for session validation
	if err := security.InitDatabase(*dbPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer security.CloseDatabase()

	// Initialize Telegram alerts (from flags OR database)
	if *telegramBot != "" && *telegramChat != "" {
		// Use command-line flags if provided
		alerts.InitTelegram(*telegramBot, *telegramChat)
	} else {
		// Try to load from database
		var botToken, chatID string
		var enabled int
		err := db.QueryRow("SELECT bot_token, chat_id, enabled FROM telegram_config WHERE id = 1").Scan(&botToken, &chatID, &enabled)
		if err == nil && enabled == 1 && botToken != "" && chatID != "" {
			alerts.InitTelegram(botToken, chatID)
			log.Println("Telegram alerts loaded from database")
		}
	}

	// Initialize ZFS pool heartbeat monitoring
	poolList, err := zfs.DiscoverPools()
	if err != nil {
		log.Printf("Warning: Failed to discover ZFS pools: %v", err)
	} else if len(poolList) > 0 {
		for _, pool := range poolList {
			heartbeat := zfs.NewPoolHeartbeat(pool.Name, pool.MountPoint, 30*time.Second)
			
			// Set up Telegram alert callback if configured
			heartbeat.SetErrorCallback(func(poolName string, err error, details map[string]string) {
				alertErr := alerts.SendAlert(alerts.TelegramAlert{
					Level:   "CRITICAL",
					Title:   fmt.Sprintf("ZFS Pool Failure: %s", poolName),
					Message: err.Error(),
					Details: details,
				})
				if alertErr != nil {
					log.Printf("Failed to send Telegram alert: %v", alertErr)
				}
			})
			
			heartbeat.Start()
			defer heartbeat.Stop()
		}
	}

	log.Printf("D-PlaneOS Daemon v%s starting...", Version)

	// Initialize WebSocket Hub for real-time monitoring
	wsHub := websocket.NewMonitorHub()
	go wsHub.Run()
	
	// Initialize Background Monitor (30s interval)
	// Broadcasts inotify stats to WebSocket clients
	bgMonitor := monitoring.NewBackgroundMonitor(30*time.Second, func(eventType string, data interface{}, level string) {
		wsHub.Broadcast(eventType, data, level)
	})
	bgMonitor.Start()
	defer bgMonitor.Stop()

	// Create router
	r := mux.NewRouter()

	// Middleware
	r.Use(loggingMiddleware)
	r.Use(sessionMiddleware)
	r.Use(rateLimitMiddleware)

	// Health check

	// ─── AUTH ROUTES (public, no session required) ───
	authHandler := handlers.NewAuthHandler(db)
	r.HandleFunc("/api/auth/login", authHandler.Login).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/auth/logout", authHandler.Logout).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/auth/check", authHandler.Check).Methods("GET")
	r.HandleFunc("/api/auth/session", authHandler.Session).Methods("GET")
	r.HandleFunc("/api/auth/change-password", authHandler.ChangePassword).Methods("POST")
	r.HandleFunc("/api/csrf", authHandler.CSRFToken).Methods("GET")

	// Session cleanup goroutine
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			authHandler.CleanExpiredSessions()
		}
	}()

	r.HandleFunc("/health", healthCheckHandler).Methods("GET")

	// ZFS handlers
	zfsHandler := handlers.NewZFSHandler()
	r.HandleFunc("/api/zfs/command", zfsHandler.HandleCommand).Methods("POST")
	r.HandleFunc("/api/zfs/pools", zfsHandler.ListPools).Methods("GET")
	r.HandleFunc("/api/zfs/datasets", zfsHandler.ListDatasets).Methods("GET")
	
	// ZFS Encryption handlers
	zfsEncryptionHandler := handlers.NewZFSEncryptionHandler()
	r.HandleFunc("/api/zfs/encryption/list", zfsEncryptionHandler.ListEncryptedDatasets).Methods("GET")
	r.HandleFunc("/api/zfs/encryption/unlock", zfsEncryptionHandler.UnlockDataset).Methods("POST")
	r.HandleFunc("/api/zfs/encryption/lock", zfsEncryptionHandler.LockDataset).Methods("POST")
	r.HandleFunc("/api/zfs/encryption/create", zfsEncryptionHandler.CreateEncryptedDataset).Methods("POST")
	r.HandleFunc("/api/zfs/encryption/change-key", zfsEncryptionHandler.ChangeKey).Methods("POST")

	// System handlers
	systemHandler := handlers.NewSystemHandler()
	r.HandleFunc("/api/system/ups", systemHandler.GetUPSStatus).Methods("GET")
	r.HandleFunc("/api/system/network", systemHandler.GetNetworkInfo).Methods("GET")
	r.HandleFunc("/api/system/logs", systemHandler.GetSystemLogs).Methods("GET")

	// Docker handlers
	dockerHandler := handlers.NewDockerHandler()
	r.HandleFunc("/api/docker/containers", dockerHandler.ListContainers).Methods("GET")
	r.HandleFunc("/api/docker/action", dockerHandler.ContainerAction).Methods("POST")
	r.HandleFunc("/api/docker/logs", dockerHandler.ContainerLogs).Methods("GET")
	// v2.1.0: Docker enhanced
	r.HandleFunc("/api/docker/update", dockerHandler.SafeUpdate).Methods("POST")
	r.HandleFunc("/api/docker/pull", dockerHandler.PullImage).Methods("POST")
	r.HandleFunc("/api/docker/remove", dockerHandler.RemoveContainer).Methods("POST")
	r.HandleFunc("/api/docker/stats", dockerHandler.ContainerStats).Methods("GET")
	r.HandleFunc("/api/docker/compose/up", dockerHandler.ComposeUp).Methods("POST")
	r.HandleFunc("/api/docker/compose/down", dockerHandler.ComposeDown).Methods("POST")
	r.HandleFunc("/api/docker/compose/status", dockerHandler.ComposeStatus).Methods("GET")

	// v2.1.0: ZFS Snapshots CRUD
	snapshotCRUDHandler := handlers.NewZFSSnapshotHandler()
	r.HandleFunc("/api/zfs/snapshots", snapshotCRUDHandler.ListSnapshots).Methods("GET")
	r.HandleFunc("/api/zfs/snapshots", snapshotCRUDHandler.CreateSnapshot).Methods("POST")
	r.HandleFunc("/api/zfs/snapshots", snapshotCRUDHandler.DestroySnapshot).Methods("DELETE")
	r.HandleFunc("/api/zfs/snapshots/rollback", snapshotCRUDHandler.RollbackSnapshot).Methods("POST")

	// v2.1.0: ZFS Replication (remote send/recv)
	replicationRemoteHandler := handlers.NewReplicationHandler()
	r.HandleFunc("/api/replication/remote", replicationRemoteHandler.ReplicateToRemote).Methods("POST")
	r.HandleFunc("/api/replication/test", replicationRemoteHandler.TestRemoteConnection).Methods("POST")

	// v2.1.0: ZFS Time Machine (browse snapshots, restore single files)
	timeMachineHandler := handlers.NewZFSTimeMachineHandler()
	r.HandleFunc("/api/timemachine/versions", timeMachineHandler.ListSnapshotVersions).Methods("GET")
	r.HandleFunc("/api/timemachine/browse", timeMachineHandler.BrowseSnapshot).Methods("GET")
	r.HandleFunc("/api/timemachine/restore", timeMachineHandler.RestoreFile).Methods("POST")

	// v2.1.0: ZFS Sandbox (ephemeral Docker environments via ZFS clone)
	sandboxHandler := handlers.NewZFSSandboxHandler()
	r.HandleFunc("/api/sandbox/create", sandboxHandler.CreateSandbox).Methods("POST")
	r.HandleFunc("/api/sandbox/list", sandboxHandler.ListSandboxes).Methods("GET")
	r.HandleFunc("/api/sandbox/destroy", sandboxHandler.DestroySandbox).Methods("DELETE")

	// v2.1.0: NixOS Config Guard (only active on NixOS systems)
	nixosGuardHandler := handlers.NewNixOSGuardHandler()
	r.HandleFunc("/api/nixos/detect", nixosGuardHandler.DetectNixOS).Methods("GET")
	r.HandleFunc("/api/nixos/validate", nixosGuardHandler.ValidateConfig).Methods("POST")
	r.HandleFunc("/api/nixos/generations", nixosGuardHandler.ListGenerations).Methods("GET")
	r.HandleFunc("/api/nixos/rollback", nixosGuardHandler.RollbackGeneration).Methods("POST")

	// v2.1.0: ZFS Health Predictor (deep monitoring, heatmap data)
	healthHandler := handlers.NewZFSHealthHandler()
	r.HandleFunc("/api/zfs/health", healthHandler.GetPoolHealth).Methods("GET")
	r.HandleFunc("/api/zfs/iostat", healthHandler.GetIOStats).Methods("GET")
	r.HandleFunc("/api/zfs/events", healthHandler.GetPoolEvents).Methods("GET")
	r.HandleFunc("/api/zfs/smart", healthHandler.GetSMARTHealth).Methods("GET")

	// v2.1.0: Pool Capacity Guardian (prevents ZFS full freeze)
	capacityHandler := handlers.NewCapacityGuardianHandler()
	r.HandleFunc("/api/zfs/capacity", capacityHandler.GetCapacityStatus).Methods("GET")
	r.HandleFunc("/api/zfs/capacity/reserve", capacityHandler.SetupReserve).Methods("POST")
	r.HandleFunc("/api/zfs/capacity/release", capacityHandler.ReleaseReserve).Methods("POST")

	// v2.1.0: Power-loss state locks
	stateLockHandler := handlers.NewStateLockHandler()
	r.HandleFunc("/api/system/stale-locks", stateLockHandler.CheckStaleLocks).Methods("GET")
	r.HandleFunc("/api/system/stale-locks/clear", stateLockHandler.ClearStaleLock).Methods("POST")

	// v2.1.0: Sandbox orphan cleanup
	r.HandleFunc("/api/sandbox/cleanup", sandboxHandler.CleanOrphanVolumes).Methods("POST")

	// v2.1.0: NixOS diff + watchdog
	r.HandleFunc("/api/nixos/diff", nixosGuardHandler.DiffGenerations).Methods("GET")
	r.HandleFunc("/api/nixos/apply", nixosGuardHandler.ApplyWithWatchdog).Methods("POST")
	r.HandleFunc("/api/nixos/confirm", nixosGuardHandler.ConfirmApply).Methods("POST")
	r.HandleFunc("/api/nixos/watchdog", nixosGuardHandler.WatchdogStatus).Methods("GET")

	// v2.1.0: Docker pre-flight check
	r.HandleFunc("/api/docker/preflight", dockerHandler.PreFlightCheck).Methods("GET")

	// ── Git Sync ──
	gitSyncHandler := handlers.NewGitSyncHandler(db)
	r.HandleFunc("/api/git-sync/config", gitSyncHandler.GetConfig).Methods("GET")
	r.HandleFunc("/api/git-sync/config", gitSyncHandler.SaveConfig).Methods("POST")
	r.HandleFunc("/api/git-sync/pull", gitSyncHandler.Pull).Methods("POST")
	r.HandleFunc("/api/git-sync/status", gitSyncHandler.Status).Methods("GET")
	r.HandleFunc("/api/git-sync/stacks", gitSyncHandler.ListStacks).Methods("GET")
	r.HandleFunc("/api/git-sync/deploy", gitSyncHandler.Deploy).Methods("POST")
	r.HandleFunc("/api/git-sync/export", gitSyncHandler.ExportContainers).Methods("POST")
	r.HandleFunc("/api/git-sync/push", gitSyncHandler.Push).Methods("POST")

	// Git-Sync: Multi-Repo + Credentials (v2.1.1)
	gitReposHandler := handlers.NewGitReposHandler(db)
	r.HandleFunc("/api/git-sync/credentials", gitReposHandler.ListCredentials).Methods("GET")
	r.HandleFunc("/api/git-sync/credentials", gitReposHandler.SaveCredential).Methods("POST")
	r.HandleFunc("/api/git-sync/credentials/test", gitReposHandler.TestCredential).Methods("POST")
	r.HandleFunc("/api/git-sync/credentials/delete", gitReposHandler.DeleteCredential).Methods("DELETE", "POST")
	r.HandleFunc("/api/git-sync/repos", gitReposHandler.ListRepos).Methods("GET")
	r.HandleFunc("/api/git-sync/repos", gitReposHandler.SaveRepo).Methods("POST")
	r.HandleFunc("/api/git-sync/repos/delete", gitReposHandler.DeleteRepo).Methods("DELETE", "POST")
	r.HandleFunc("/api/git-sync/repos/pull", gitReposHandler.PullRepo).Methods("POST")
	r.HandleFunc("/api/git-sync/repos/push", gitReposHandler.PushRepo).Methods("POST")
	r.HandleFunc("/api/git-sync/repos/deploy", gitReposHandler.DeployRepo).Methods("POST")
	r.HandleFunc("/api/git-sync/repos/export", gitReposHandler.ExportToRepo).Methods("POST")
	gitSyncHandler.StartAutoSync()

	// v2.1.0: Audit log rotation
	auditRotationHandler := handlers.NewAuditRotationHandler()
	r.HandleFunc("/api/system/audit/rotate", auditRotationHandler.RotateAuditLogs).Methods("POST")
	r.HandleFunc("/api/system/audit/stats", auditRotationHandler.GetAuditStats).Methods("GET")

	// v2.1.0: Zombie disk watcher
	zombieHandler := handlers.NewZombieWatcherHandler()
	r.HandleFunc("/api/zfs/disk-latency", zombieHandler.CheckDiskLatency).Methods("GET")

	// v2.1.0: LDAP Circuit Breaker
	r.HandleFunc("/api/ldap/circuit-breaker", handlers.GetCircuitBreakerStatus).Methods("GET")
	r.HandleFunc("/api/ldap/circuit-breaker/reset", handlers.ResetCircuitBreaker).Methods("POST")

	// v2.1.0: ZFS Scrub management
	r.HandleFunc("/api/zfs/scrub/start", handlers.StartScrub).Methods("POST")
	r.HandleFunc("/api/zfs/scrub/stop", handlers.StopScrub).Methods("POST")
	r.HandleFunc("/api/zfs/scrub/status", handlers.GetScrubStatus).Methods("GET")

	// v2.1.0: VDEV / Pool expansion
	r.HandleFunc("/api/zfs/pool/add-vdev", handlers.AddVdevToPool).Methods("POST")
	r.HandleFunc("/api/zfs/pool/remove-device", handlers.RemoveCacheOrLog).Methods("POST")
	r.HandleFunc("/api/zfs/pool/replace", handlers.ReplaceDisk).Methods("POST")

	// v2.1.0: Dataset quotas
	r.HandleFunc("/api/zfs/dataset/quota", handlers.SetDatasetQuota).Methods("POST")
	r.HandleFunc("/api/zfs/dataset/quota", handlers.GetDatasetQuota).Methods("GET")

	// v2.1.0: S.M.A.R.T. tests
	r.HandleFunc("/api/zfs/smart/test", handlers.RunSMARTTest).Methods("POST")
	r.HandleFunc("/api/zfs/smart/results", handlers.GetSMARTTestResults).Methods("GET")

	// v2.1.0: ZFS delegation (zfs allow)
	r.HandleFunc("/api/zfs/delegation", handlers.SetZFSDelegation).Methods("POST")
	r.HandleFunc("/api/zfs/delegation", handlers.GetZFSDelegation).Methods("GET")
	r.HandleFunc("/api/zfs/delegation/revoke", handlers.RevokeZFSDelegation).Methods("POST")

	// v2.1.0: Network rollback
	r.HandleFunc("/api/network/apply", handlers.ApplyNetworkWithRollback).Methods("POST")
	r.HandleFunc("/api/network/confirm", handlers.ConfirmNetwork).Methods("POST")

	// v2.1.0: SMB VFS modules
	r.HandleFunc("/api/smb/vfs", handlers.GetSMBVFSConfig).Methods("GET")
	r.HandleFunc("/api/smb/vfs", handlers.SetSMBVFSConfig).Methods("POST")

	// v2.1.0: VLAN management
	r.HandleFunc("/api/network/vlan", handlers.ListVLANs).Methods("GET")
	r.HandleFunc("/api/network/vlan", handlers.CreateVLAN).Methods("POST")
	r.HandleFunc("/api/network/vlan", handlers.DeleteVLAN).Methods("DELETE")

	// v2.1.0: Link Aggregation / Bonding
	r.HandleFunc("/api/network/bond", handlers.CreateBond).Methods("POST")

	// v2.1.0: NTP configuration
	r.HandleFunc("/api/system/ntp", handlers.GetNTPStatus).Methods("GET")
	r.HandleFunc("/api/system/ntp", handlers.SetNTPServers).Methods("POST")

	// Shares handlers (config management)
	r.HandleFunc("/api/shares/smb/reload", handlers.ReloadSMBConfig).Methods("POST")
	r.HandleFunc("/api/shares/smb/test", handlers.TestSMBConfig).Methods("POST")
	r.HandleFunc("/api/shares/nfs/reload", handlers.ReloadNFSExports).Methods("POST")
	r.HandleFunc("/api/shares/nfs/list", handlers.ListNFSExports).Methods("GET")

	// Shares CRUD handlers
	shareCRUDHandler := handlers.NewShareCRUDHandler(db, *smbConfPath)
	r.HandleFunc("/api/shares/list", shareCRUDHandler.HandleShares).Methods("GET")
	r.HandleFunc("/api/shares", shareCRUDHandler.HandleShares).Methods("GET", "POST")

	// User & Group CRUD handlers
	userGroupHandler := handlers.NewUserGroupHandler(db)
	r.HandleFunc("/api/rbac/users", userGroupHandler.HandleUsers).Methods("GET", "POST")
	r.HandleFunc("/api/rbac/groups", userGroupHandler.HandleGroups).Methods("GET", "POST")
	r.HandleFunc("/api/users/create", userGroupHandler.HandleUsers).Methods("POST")

	// System status, profile, preflight, setup handlers
	systemStatusHandler := handlers.NewSystemStatusHandler(db)
	r.HandleFunc("/api/system/status", systemStatusHandler.HandleStatus).Methods("GET")
	r.HandleFunc("/api/system/profile", systemStatusHandler.HandleProfile).Methods("GET")
	r.HandleFunc("/api/system/settings", systemStatusHandler.HandleSettings).Methods("GET", "POST")
	r.HandleFunc("/api/system/preflight", systemStatusHandler.HandlePreflight).Methods("GET")
	r.HandleFunc("/api/system/zfs-gate-status", systemStatusHandler.HandleZFSGateStatus).Methods("GET")
	// /api/status is an alias for /api/system/status (used by dashboard ECC check)
	r.HandleFunc("/api/status", systemStatusHandler.HandleStatus).Methods("GET")
	r.HandleFunc("/api/system/setup-complete", systemStatusHandler.HandleSetupComplete).Methods("POST")
	r.HandleFunc("/api/system/metrics", handlers.HandleSystemMetrics).Methods("GET")

	// Disk discovery (setup wizard)
	r.HandleFunc("/api/system/disks", handlers.HandleDiskDiscovery).Methods("GET")
	r.HandleFunc("/api/system/pool/create", handlers.HandlePoolCreate).Methods("POST")
	
	// Files handlers
	filesHandler := handlers.NewFilesExtendedHandler()
	r.HandleFunc("/api/files/list", filesHandler.ListFiles).Methods("GET")
	r.HandleFunc("/api/files/properties", filesHandler.GetFileProperties).Methods("GET")
	r.HandleFunc("/api/files/rename", filesHandler.RenameFile).Methods("POST")
	r.HandleFunc("/api/files/copy", filesHandler.CopyFile).Methods("POST")
	r.HandleFunc("/api/files/upload", filesHandler.UploadChunk).Methods("POST")
	r.HandleFunc("/api/files/mkdir", handlers.CreateDirectory).Methods("POST")
	r.HandleFunc("/api/files/delete", handlers.DeletePath).Methods("POST")
	r.HandleFunc("/api/files/chown", handlers.ChangeOwnership).Methods("POST")
	r.HandleFunc("/api/files/chmod", handlers.ChangePermissions).Methods("POST")
	
	// Backup handlers
	r.HandleFunc("/api/backup/rsync", handlers.ExecuteRsync).Methods("POST")
	
	// Replication handlers
	r.HandleFunc("/api/replication/send", handlers.ZFSSend).Methods("POST")
	r.HandleFunc("/api/replication/send-incremental", handlers.ZFSSendIncremental).Methods("POST")
	r.HandleFunc("/api/replication/receive", handlers.ZFSReceive).Methods("POST")
	
	// Settings handlers
	settingsHandler := handlers.NewSettingsHandler(db)
	r.HandleFunc("/api/settings/telegram", settingsHandler.GetTelegramConfig).Methods("GET")
	r.HandleFunc("/api/settings/telegram", settingsHandler.SaveTelegramConfig).Methods("POST")
	r.HandleFunc("/api/settings/telegram/test", settingsHandler.TestTelegramConfig).Methods("POST")
	
	// Removable Media handlers
	removableHandler := handlers.NewRemovableMediaHandler()
	r.HandleFunc("/api/removable/list", removableHandler.ListDevices).Methods("GET")
	r.HandleFunc("/api/removable/mount", removableHandler.MountDevice).Methods("POST")
	r.HandleFunc("/api/removable/unmount", removableHandler.UnmountDevice).Methods("POST")
	r.HandleFunc("/api/removable/eject", removableHandler.EjectDevice).Methods("POST")
	
	// Monitoring handlers
	monitoringHandler := handlers.NewMonitoringHandler()
	r.HandleFunc("/api/monitoring/inotify", monitoringHandler.GetInotifyStats).Methods("GET")

	// LDAP / Active Directory handlers (v2.0.0)
	ldapHandler := handlers.NewLDAPHandler(db)
	r.HandleFunc("/api/ldap/config", ldapHandler.GetConfig).Methods("GET")
	r.HandleFunc("/api/ldap/config", ldapHandler.SaveConfig).Methods("POST")
	r.HandleFunc("/api/ldap/test", ldapHandler.TestConnection).Methods("POST")
	r.HandleFunc("/api/ldap/status", ldapHandler.GetStatus).Methods("GET")
	r.HandleFunc("/api/ldap/sync", ldapHandler.TriggerSync).Methods("POST")
	r.HandleFunc("/api/ldap/search-user", ldapHandler.SearchUser).Methods("POST")
	r.HandleFunc("/api/ldap/mappings", ldapHandler.GetMappings).Methods("GET")
	r.HandleFunc("/api/ldap/mappings", ldapHandler.AddMapping).Methods("POST")
	r.HandleFunc("/api/ldap/mappings", ldapHandler.DeleteMapping).Methods("DELETE")
	r.HandleFunc("/api/ldap/sync-log", ldapHandler.GetSyncLog).Methods("GET")

	// RBAC routes
	r.HandleFunc("/api/rbac/roles", handlers.HandleListRoles).Methods("GET")
	r.HandleFunc("/api/rbac/roles", handlers.HandleCreateRole).Methods("POST")
	r.HandleFunc("/api/rbac/roles/{id}", handlers.HandleGetRole).Methods("GET")
	r.HandleFunc("/api/rbac/roles/{id}", handlers.HandleUpdateRole).Methods("PUT")
	r.HandleFunc("/api/rbac/roles/{id}", handlers.HandleDeleteRole).Methods("DELETE")
	r.HandleFunc("/api/rbac/roles/{id}/permissions", handlers.HandleGetRolePermissions).Methods("GET")
	r.HandleFunc("/api/rbac/roles/{id}/permissions", handlers.HandleAssignPermissionToRole).Methods("POST")
	r.HandleFunc("/api/rbac/roles/{id}/permissions/{permissionId}", handlers.HandleRemovePermissionFromRole).Methods("DELETE")
	r.HandleFunc("/api/rbac/permissions", handlers.HandleListPermissions).Methods("GET")
	r.HandleFunc("/api/rbac/users/{id}/roles", handlers.HandleGetUserRoles).Methods("GET")
	r.HandleFunc("/api/rbac/users/{id}/roles", handlers.HandleAssignRoleToUser).Methods("POST")
	r.HandleFunc("/api/rbac/users/{id}/roles/{roleId}", handlers.HandleRemoveRoleFromUser).Methods("DELETE")
	r.HandleFunc("/api/rbac/users/{id}/permissions", handlers.HandleGetUserPermissions).Methods("GET")
	r.HandleFunc("/api/rbac/me/permissions", handlers.HandleGetMyPermissions).Methods("GET")
	r.HandleFunc("/api/rbac/me/roles", handlers.HandleGetMyRoles).Methods("GET")
	r.HandleFunc("/api/rbac/check", handlers.HandleCheckPermission).Methods("GET")

	// Snapshot Scheduler (v2.0.0)
	snapScheduleHandler := handlers.NewSnapshotScheduleHandler()
	r.HandleFunc("/api/snapshots/schedules", snapScheduleHandler.ListSchedules).Methods("GET")
	r.HandleFunc("/api/snapshots/schedules", snapScheduleHandler.SaveSchedules).Methods("POST")
	r.HandleFunc("/api/snapshots/run-now", snapScheduleHandler.RunNow).Methods("POST")

	// ACL Management (v2.0.0)
	aclHandler := handlers.NewACLHandler()
	r.HandleFunc("/api/acl/get", aclHandler.GetACL).Methods("GET")
	r.HandleFunc("/api/acl/set", aclHandler.SetACL).Methods("POST")

	// Metrics / Reporting (v2.0.0)
	metricsHandler := handlers.NewMetricsHandler()
	r.HandleFunc("/api/metrics/current", metricsHandler.GetCurrentMetrics).Methods("GET")
	r.HandleFunc("/api/metrics/history", metricsHandler.GetHistory).Methods("GET")

	// Firewall (v2.0.0)
	firewallHandler := handlers.NewFirewallHandler()
	r.HandleFunc("/api/firewall/status", firewallHandler.GetStatus).Methods("GET")
	r.HandleFunc("/api/firewall/rule", firewallHandler.SetRule).Methods("POST")

	// SSL/TLS Certificates (v2.0.0)
	certHandler := handlers.NewCertHandler()
	r.HandleFunc("/api/certs/list", certHandler.ListCerts).Methods("GET")
	r.HandleFunc("/api/certs/generate", certHandler.GenerateSelfSigned).Methods("POST")
	r.HandleFunc("/api/certs/activate", certHandler.ActivateCert).Methods("POST")

	// Trash / Recycle Bin (v2.0.0)
	trashHandler := handlers.NewTrashHandler()
	r.HandleFunc("/api/trash/list", trashHandler.ListTrash).Methods("GET")
	r.HandleFunc("/api/trash/move", trashHandler.MoveToTrash).Methods("POST")
	r.HandleFunc("/api/trash/restore", trashHandler.RestoreFromTrash).Methods("POST")
	r.HandleFunc("/api/trash/empty", trashHandler.EmptyTrash).Methods("POST")

	// Power Management (v2.0.0)
	powerHandler := handlers.NewPowerMgmtHandler()
	r.HandleFunc("/api/power/disks", powerHandler.GetDiskStatus).Methods("GET")
	r.HandleFunc("/api/power/spindown", powerHandler.SetSpindown).Methods("POST")
	r.HandleFunc("/api/power/spindown-now", powerHandler.SpindownNow).Methods("POST")

	// SMTP Email Alerting
	r.HandleFunc("/api/alerts/smtp", handlers.GetSMTPConfig).Methods("GET")
	r.HandleFunc("/api/alerts/smtp", handlers.SaveSMTPConfig).Methods("POST")
	r.HandleFunc("/api/alerts/smtp/test", handlers.TestSMTP).Methods("POST")

	// ZFS Scrub Scheduler
	r.HandleFunc("/api/zfs/scrub/schedule", handlers.GetScrubSchedules).Methods("GET")
	r.HandleFunc("/api/zfs/scrub/schedule", handlers.SaveScrubSchedules).Methods("POST")

	// Start background monitors
	handlers.StartScrubMonitor()

	// WebSocket for real-time monitoring
	wsHandler := handlers.NewWebSocketHandler(wsHub)
	r.HandleFunc("/ws/monitor", wsHandler.HandleMonitor)

	// Create server
	srv := &http.Server{
		Addr:         *listenAddr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start background monitors
	handlers.StartCapacityMonitor()
	log.Println("Capacity guardian started (checks every 5 min)")

	// Start server in goroutine
	go func() {
		log.Printf("Listening on %s", *listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Audit startup
	audit.Log(audit.AuditLog{
		Level:   audit.LevelInfo,
		Command: "DAEMON_START",
		Success: true,
		Metadata: map[string]interface{}{
			"version": Version,
			"listen":  *listenAddr,
		},
	})

	// Wait for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop

	log.Println("Shutting down gracefully...")

	// Audit shutdown
	audit.Log(audit.AuditLog{
		Level:   audit.LevelInfo,
		Command: "DAEMON_STOP",
		Success: true,
	})

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","version":"%s"}`, Version)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s %v", r.RemoteAddr, r.Method, r.URL.Path, time.Since(start))
	})
}

// Thread-safe rate limiting middleware (per IP)
var (
	rateLimitMu   sync.Mutex
	requestCounts = make(map[string][]time.Time)
	maxRequests   = 100
	timeWindow    = time.Minute
)

func rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr

		rateLimitMu.Lock()
		now := time.Now()
		if timestamps, exists := requestCounts[ip]; exists {
			// Remove old timestamps
			var recent []time.Time
			for _, t := range timestamps {
				if now.Sub(t) < timeWindow {
					recent = append(recent, t)
				}
			}
			requestCounts[ip] = recent

			// Check rate limit
			if len(recent) >= maxRequests {
				rateLimitMu.Unlock()
				audit.LogSecurityEvent(
					fmt.Sprintf("Rate limit exceeded: %d requests in %v", len(recent), timeWindow),
					r.Header.Get("X-User"),
					ip,
				)
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}
		}

		// Add current request
		requestCounts[ip] = append(requestCounts[ip], now)
		rateLimitMu.Unlock()

		next.ServeHTTP(w, r)
	})
}

// Session validation middleware
func sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip validation for public endpoints (auth, csrf, health)
		if r.URL.Path == "/health" ||
			strings.HasPrefix(r.URL.Path, "/api/auth/") ||
			r.URL.Path == "/api/csrf" {
			next.ServeHTTP(w, r)
			return
		}

		sessionID := r.Header.Get("X-Session-ID")
		user := r.Header.Get("X-User")

		if sessionID == "" || user == "" {
			audit.LogSecurityEvent("Missing session or user header", user, r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Validate session and get user details
		sessionUser, err := security.ValidateSessionAndGetUser(sessionID)
		if err != nil {
			// Fall back to format validation only
			if !security.IsValidSessionToken(sessionID) {
				audit.LogSecurityEvent("Invalid session format", user, r.RemoteAddr)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			// Token format OK but DB lookup failed — proceed without context user
			next.ServeHTTP(w, r)
			return
		}

		// Verify header user matches session user
		if sessionUser.Username != user {
			audit.LogSecurityEvent("Session user mismatch", user, r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Set user in context for downstream handlers (RBAC /me/* endpoints)
		ctx := context.WithValue(r.Context(), middleware.UserContextKey, &middleware.User{
			ID:       sessionUser.ID,
			Username: sessionUser.Username,
			Email:    sessionUser.Email,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
