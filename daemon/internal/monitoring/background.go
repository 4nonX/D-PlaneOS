package monitoring

import (
	"log"
	"time"
)

// BackgroundMonitor runs periodic checks and sends alerts
type BackgroundMonitor struct {
	interval      time.Duration
	alertCallback func(eventType string, data interface{}, level string)
	stopChan      chan bool
}

// NewBackgroundMonitor creates a new background monitor
func NewBackgroundMonitor(interval time.Duration, alertCallback func(string, interface{}, string)) *BackgroundMonitor {
	return &BackgroundMonitor{
		interval:      interval,
		alertCallback: alertCallback,
		stopChan:      make(chan bool),
	}
}

// Start begins the monitoring loop
func (m *BackgroundMonitor) Start() {
	go m.run()
}

// Stop halts the monitoring loop
func (m *BackgroundMonitor) Stop() {
	m.stopChan <- true
}

func (m *BackgroundMonitor) run() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.check()
		case <-m.stopChan:
			log.Println("Background monitor stopped")
			return
		}
	}
}

func (m *BackgroundMonitor) check() {
	// Check inotify stats
	stats, err := GetInotifyStats()
	if err != nil {
		log.Printf("Error getting inotify stats: %v", err)
		return
	}

	// Send event based on status
	if stats.Critical {
		m.alertCallback("inotify_status", stats, "critical")
		log.Printf("CRITICAL: Inotify at %.1f%% (%d/%d watches)", stats.Percent, stats.Used, stats.Limit)
	} else if stats.Warning {
		m.alertCallback("inotify_status", stats, "warning")
		log.Printf("WARNING: Inotify at %.1f%% (%d/%d watches)", stats.Percent, stats.Used, stats.Limit)
	} else {
		// Send info-level update every check (for UI dashboard)
		m.alertCallback("inotify_status", stats, "info")
	}
}

// CheckMountStatus verifies all registered ZFS mounts
func (m *BackgroundMonitor) checkMountStatus() {
	// This would be called from the monitoring loop
	// Check for mount guard files
	// Alert via WebSocket if missing
}
