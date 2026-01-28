#!/bin/bash
# D-PlaneOS Automated Monitoring
# Collects metrics and checks alerts every 5 minutes

# Collect system metrics
curl -s -X POST http://localhost/api/system/metrics.php \
  -H "Content-Type: application/json" \
  -d '{"action":"collect"}' > /dev/null

# Check for alerts
curl -s -X POST http://localhost/api/system/alerts.php \
  -H "Content-Type: application/json" \
  -d '{"action":"check"}' > /dev/null

# Cleanup old metrics (once daily)
if [ $(date +%H:%M) = "03:00" ]; then
  curl -s -X POST http://localhost/api/system/metrics.php \
    -H "Content-Type: application/json" \
    -d '{"action":"cleanup"}' > /dev/null
fi
