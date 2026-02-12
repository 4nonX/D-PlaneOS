#!/usr/bin/env python3
"""
D-PlaneOS Alerting System v2.9.0
Email, Push notifications, and webhooks for critical events
"""

import smtplib
import json
import subprocess
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from datetime import datetime
from pathlib import Path
import requests

class AlertingSystem:
    def __init__(self):
        self.config = self.load_config()
    
    def load_config(self):
        """Load alerting configuration"""
        config_file = Path('/etc/dplaneos/alerting.json')
        
        if config_file.exists():
            with open(config_file) as f:
                return json.load(f)
        
        # Default config
        return {
            'email': {
                'enabled': False,
                'smtp_server': 'localhost',
                'smtp_port': 587,
                'smtp_user': '',
                'smtp_password': '',
                'from_addr': 'dplaneos@localhost',
                'to_addrs': [],
                'use_tls': True
            },
            'pushover': {
                'enabled': False,
                'user_key': '',
                'api_token': ''
            },
            'ntfy': {
                'enabled': False,
                'server': 'https://ntfy.sh',
                'topic': 'dplaneos'
            },
            'webhook': {
                'enabled': False,
                'url': '',
                'method': 'POST'
            },
            'alerts': {
                'disk_added': True,
                'disk_removed': True,
                'disk_temp_warning': True,
                'disk_smart_failure': True,
                'pool_degraded': True,
                'resilver_started': True,
                'resilver_completed': True,
                'ups_on_battery': True,
                'ups_low_battery': True,
                'system_high_temp': True
            }
        }
    
    def should_alert(self, event_type):
        """Check if this event type should trigger an alert"""
        return self.config['alerts'].get(event_type, False)
    
    def send_email(self, subject, body, priority='normal'):
        """Send email alert"""
        if not self.config['email']['enabled']:
            return False
        
        try:
            msg = MIMEMultipart()
            msg['From'] = self.config['email']['from_addr']
            msg['To'] = ', '.join(self.config['email']['to_addrs'])
            msg['Subject'] = f"[D-PlaneOS] {subject}"
            
            # Add priority headers
            if priority == 'critical':
                msg['X-Priority'] = '1'
                msg['Importance'] = 'high'
            
            # HTML body with styling
            html_body = f"""
            <html>
            <body style="font-family: Arial, sans-serif; margin: 20px;">
                <div style="border-left: 4px solid {'#ef4444' if priority == 'critical' else '#3b82f6'}; padding-left: 16px;">
                    <h2 style="color: {'#dc2626' if priority == 'critical' else '#1e40af'};">{subject}</h2>
                    <p style="color: #374151; line-height: 1.6;">{body}</p>
                    <hr style="border: none; border-top: 1px solid #e5e7eb; margin: 20px 0;">
                    <p style="font-size: 12px; color: #6b7280;">
                        Time: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}<br>
                        System: D-PlaneOS v2.9.0
                    </p>
                </div>
            </body>
            </html>
            """
            
            msg.attach(MIMEText(html_body, 'html'))
            
            # Connect and send
            smtp_config = self.config['email']
            
            if smtp_config['use_tls']:
                server = smtplib.SMTP(smtp_config['smtp_server'], smtp_config['smtp_port'])
                server.starttls()
            else:
                server = smtplib.SMTP(smtp_config['smtp_server'], smtp_config['smtp_port'])
            
            if smtp_config['smtp_user']:
                server.login(smtp_config['smtp_user'], smtp_config['smtp_password'])
            
            server.send_message(msg)
            server.quit()
            
            return True
        
        except Exception as e:
            print(f"Email alert failed: {e}")
            return False
    
    def send_pushover(self, title, message, priority=0):
        """Send Pushover notification"""
        if not self.config['pushover']['enabled']:
            return False
        
        try:
            response = requests.post('https://api.pushover.net/1/messages.json', data={
                'token': self.config['pushover']['api_token'],
                'user': self.config['pushover']['user_key'],
                'title': title,
                'message': message,
                'priority': priority  # -2=lowest, -1=low, 0=normal, 1=high, 2=emergency
            }, timeout=10)
            
            return response.status_code == 200
        
        except Exception as e:
            print(f"Pushover alert failed: {e}")
            return False
    
    def send_ntfy(self, title, message, priority='default', tags=[]):
        """Send ntfy.sh notification"""
        if not self.config['ntfy']['enabled']:
            return False
        
        try:
            url = f"{self.config['ntfy']['server']}/{self.config['ntfy']['topic']}"
            
            headers = {
                'Title': title,
                'Priority': priority,  # min, low, default, high, urgent
                'Tags': ','.join(tags)
            }
            
            response = requests.post(url, data=message, headers=headers, timeout=10)
            
            return response.status_code == 200
        
        except Exception as e:
            print(f"ntfy alert failed: {e}")
            return False
    
    def send_webhook(self, event_type, data):
        """Send webhook notification"""
        if not self.config['webhook']['enabled']:
            return False
        
        try:
            payload = {
                'event_type': event_type,
                'timestamp': datetime.now().isoformat(),
                'data': data,
                'system': 'D-PlaneOS v2.9.0'
            }
            
            if self.config['webhook']['method'].upper() == 'POST':
                response = requests.post(
                    self.config['webhook']['url'],
                    json=payload,
                    timeout=10
                )
            else:
                response = requests.get(
                    self.config['webhook']['url'],
                    params=payload,
                    timeout=10
                )
            
            return response.status_code == 200
        
        except Exception as e:
            print(f"Webhook alert failed: {e}")
            return False
    
    def alert(self, event_type, title, message, priority='normal', data={}):
        """Send alert via all configured channels"""
        
        if not self.should_alert(event_type):
            return
        
        results = {
            'email': False,
            'pushover': False,
            'ntfy': False,
            'webhook': False
        }
        
        # Determine priority levels for different systems
        pushover_priority = 0
        if priority == 'critical':
            pushover_priority = 2  # Emergency
        elif priority == 'warning':
            pushover_priority = 1  # High
        
        ntfy_priority = 'default'
        if priority == 'critical':
            ntfy_priority = 'urgent'
        elif priority == 'warning':
            ntfy_priority = 'high'
        
        ntfy_tags = []
        if priority == 'critical':
            ntfy_tags = ['rotating_light', 'warning']
        elif priority == 'warning':
            ntfy_tags = ['warning']
        
        # Send via all channels
        results['email'] = self.send_email(title, message, priority)
        results['pushover'] = self.send_pushover(title, message, pushover_priority)
        results['ntfy'] = self.send_ntfy(title, message, ntfy_priority, ntfy_tags)
        results['webhook'] = self.send_webhook(event_type, {
            'title': title,
            'message': message,
            'priority': priority,
            **data
        })
        
        return results


# Predefined alert templates
class AlertTemplates:
    @staticmethod
    def disk_added(disk, model, size):
        return {
            'title': f'New Disk Detected: {disk}',
            'message': f'A new disk has been connected:\n\nDevice: {disk}\nModel: {model}\nSize: {size}',
            'priority': 'normal'
        }
    
    @staticmethod
    def disk_removed(disk):
        return {
            'title': f'Disk Removed: {disk}',
            'message': f'Disk {disk} has been disconnected from the system.',
            'priority': 'warning'
        }
    
    @staticmethod
    def disk_temp_warning(disk, temperature):
        return {
            'title': f'High Disk Temperature: {disk}',
            'message': f'Disk {disk} temperature is {temperature}°C\n\nRecommended action: Check cooling system.',
            'priority': 'warning'
        }
    
    @staticmethod
    def disk_smart_failure(disk):
        return {
            'title': f'SMART Failure: {disk}',
            'message': f'⚠️ CRITICAL: Disk {disk} has failed SMART checks!\n\nIMMEDIATE ACTION REQUIRED:\n- Backup data immediately\n- Replace disk as soon as possible\n- Check ZFS pool status',
            'priority': 'critical'
        }
    
    @staticmethod
    def pool_degraded(pool, status):
        return {
            'title': f'ZFS Pool Degraded: {pool}',
            'message': f'⚠️ Pool {pool} is in {status} state!\n\nCheck pool status and replace failed disks.',
            'priority': 'critical'
        }
    
    @staticmethod
    def resilver_started(pool):
        return {
            'title': f'ZFS Resilver Started: {pool}',
            'message': f'ZFS resilver operation has started on pool {pool}.\n\nThis may take several hours depending on pool size.',
            'priority': 'normal'
        }
    
    @staticmethod
    def resilver_completed(pool):
        return {
            'title': f'ZFS Resilver Completed: {pool}',
            'message': f'✓ ZFS resilver operation completed successfully on pool {pool}.',
            'priority': 'normal'
        }
    
    @staticmethod
    def ups_on_battery(runtime_seconds):
        hours = runtime_seconds // 3600
        minutes = (runtime_seconds % 3600) // 60
        
        return {
            'title': 'UPS On Battery',
            'message': f'⚠️ Power failure detected!\n\nSystem is running on UPS battery.\nEstimated runtime: {hours}h {minutes}m\n\nSystem will shutdown automatically if power is not restored.',
            'priority': 'warning'
        }
    
    @staticmethod
    def ups_low_battery():
        return {
            'title': 'UPS Low Battery - Shutting Down',
            'message': f'⚠️ CRITICAL: UPS battery critically low!\n\nSystem is shutting down NOW to prevent data loss.',
            'priority': 'critical'
        }
    
    @staticmethod
    def system_high_temp(temp):
        return {
            'title': 'High System Temperature',
            'message': f'⚠️ System temperature is {temp}°C\n\nCheck cooling system and airflow.',
            'priority': 'warning'
        }


# CLI interface
if __name__ == '__main__':
    import sys
    
    if len(sys.argv) < 3:
        print("Usage: dplaneos-alert.py <event_type> <title> [message] [priority]")
        sys.exit(1)
    
    event_type = sys.argv[1]
    title = sys.argv[2]
    message = sys.argv[3] if len(sys.argv) > 3 else ''
    priority = sys.argv[4] if len(sys.argv) > 4 else 'normal'
    
    alerting = AlertingSystem()
    results = alerting.alert(event_type, title, message, priority)
    
    print(f"Alert sent: {results}")
