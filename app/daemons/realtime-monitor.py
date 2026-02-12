#!/usr/bin/env python3
"""
D-PlaneOS v2.9.0 - Real-time Event Daemon
Provides WebSocket server for live system updates and hardware event monitoring
"""

import asyncio
import json
import subprocess
import re
import time
from datetime import datetime
from pathlib import Path
from typing import Dict, Set, Optional
import websockets
import pyudev
import psutil
import sys

# Add alerting system to path
sys.path.insert(0, str(Path(__file__).parent))

try:
    from alerting_system import AlertingSystem, AlertTemplates
    ALERTING_AVAILABLE = True
except ImportError:
    ALERTING_AVAILABLE = False
    print("Warning: Alerting system not available")


def validate_session(session_id):
    """Validate PHP session ID"""
    if not session_id:
        return False
    
    # Check PHP session file exists
    session_file = Path(f'/var/lib/php/sessions/sess_{session_id}')
    
    # Alternative paths for different distros
    if not session_file.exists():
        session_file = Path(f'/tmp/sess_{session_id}')
    if not session_file.exists():
        session_file = Path(f'/var/tmp/sess_{session_id}')
    
    if not session_file.exists():
        return False
    
    # Check if session is not expired (modified within last hour)
    try:
        mtime = session_file.stat().st_mtime
        if time.time() - mtime > 3600:  # 1 hour
            return False
        return True
    except:
        return False

# Configuration
WS_PORT = 8081
WS_HOST = '0.0.0.0'

# Connected clients
clients: Set[websockets.WebSocketServerProtocol] = set()

# Alerting system
alerting = AlertingSystem() if ALERTING_AVAILABLE else None

# System state cache
system_state = {
    'cpu': {'usage': 0, 'temp': 0},
    'memory': {'used': 0, 'total': 0, 'percent': 0},
    'disks': {},
    'zfs_pools': {},
    'docker': {'containers': 0, 'running': 0},
    'network': {'interfaces': {}}
}


class SystemMonitor:
    """Monitors system state and detects changes"""
    
    def __init__(self):
        self.last_disk_state = {}
        self.last_pool_state = {}
        
    async def get_cpu_stats(self) -> Dict:
        """Get CPU usage and temperature"""
        cpu_percent = psutil.cpu_percent(interval=1)
        
        # Try to get CPU temperature
        temp = 0
        try:
            temp_output = subprocess.run(
                ['sensors', '-A', '-u'],
                capture_output=True, text=True, timeout=2
            )
            if temp_output.returncode == 0:
                for line in temp_output.stdout.split('\n'):
                    if 'temp1_input' in line or 'Core 0' in line:
                        match = re.search(r'([\d.]+)', line)
                        if match:
                            temp = float(match.group(1))
                            break
        except (subprocess.TimeoutExpired, FileNotFoundError):
            pass
        
        return {'usage': round(cpu_percent, 1), 'temp': round(temp, 1)}
    
    async def get_memory_stats(self) -> Dict:
        """Get memory usage"""
        mem = psutil.virtual_memory()
        return {
            'used': round(mem.used / (1024**3), 2),
            'total': round(mem.total / (1024**3), 2),
            'percent': round(mem.percent, 1)
        }
    
    async def get_disk_stats(self) -> Dict:
        """Get disk information"""
        disks = {}
        
        try:
            # Get block devices
            result = subprocess.run(
                ['lsblk', '-J', '-o', 'NAME,SIZE,TYPE,MOUNTPOINT,MODEL,SERIAL'],
                capture_output=True, text=True, timeout=5
            )
            
            if result.returncode == 0:
                data = json.loads(result.stdout)
                for device in data.get('blockdevices', []):
                    if device['type'] == 'disk':
                        disk_name = device['name']
                        
                        # Get SMART status
                        smart_status = 'UNKNOWN'
                        try:
                            smart_result = subprocess.run(
                                ['smartctl', '-H', f'/dev/{disk_name}'],
                                capture_output=True, text=True, timeout=3
                            )
                            if 'PASSED' in smart_result.stdout:
                                smart_status = 'PASSED'
                            elif 'FAILED' in smart_result.stdout:
                                smart_status = 'FAILED'
                        except:
                            pass
                        
                        # Get temperature
                        temp = None
                        try:
                            temp_result = subprocess.run(
                                ['smartctl', '-A', f'/dev/{disk_name}'],
                                capture_output=True, text=True, timeout=3
                            )
                            for line in temp_result.stdout.split('\n'):
                                if 'Temperature_Celsius' in line or 'Airflow_Temperature' in line:
                                    parts = line.split()
                                    if len(parts) >= 10:
                                        temp = int(parts[9])
                                        break
                        except:
                            pass
                        
                        disks[disk_name] = {
                            'size': device.get('size', ''),
                            'model': device.get('model', 'Unknown'),
                            'serial': device.get('serial', ''),
                            'smart_status': smart_status,
                            'temperature': temp,
                            'mountpoint': device.get('mountpoint', '')
                        }
        except Exception as e:
            print(f"Error getting disk stats: {e}")
        
        return disks
    
    async def get_zfs_pool_stats(self) -> Dict:
        """Get ZFS pool information including resilver progress"""
        pools = {}
        
        try:
            # Get pool list
            result = subprocess.run(
                ['zpool', 'list', '-H', '-o', 'name,size,alloc,free,cap,health'],
                capture_output=True, text=True, timeout=5
            )
            
            if result.returncode == 0:
                for line in result.stdout.strip().split('\n'):
                    if not line:
                        continue
                    
                    parts = line.split('\t')
                    if len(parts) >= 6:
                        pool_name = parts[0]
                        
                        # Get detailed status including resilver
                        status_result = subprocess.run(
                            ['zpool', 'status', pool_name],
                            capture_output=True, text=True, timeout=5
                        )
                        
                        resilver_info = None
                        scrub_info = None
                        
                        if status_result.returncode == 0:
                            status_text = status_result.stdout
                            
                            # Check for resilver
                            if 'resilver in progress' in status_text.lower():
                                # Extract progress
                                for line in status_text.split('\n'):
                                    if 'scanned' in line.lower() and '%' in line:
                                        match = re.search(r'([\d.]+)%.*?(\d+h\d+m|\d+m\d+s|\d+s)', line)
                                        if match:
                                            resilver_info = {
                                                'progress': float(match.group(1)),
                                                'time_remaining': match.group(2) if len(match.groups()) > 1 else 'calculating'
                                            }
                            
                            # Check for scrub
                            if 'scrub in progress' in status_text.lower():
                                for line in status_text.split('\n'):
                                    if 'scanned' in line.lower() and '%' in line:
                                        match = re.search(r'([\d.]+)%.*?(\d+h\d+m|\d+m\d+s|\d+s)', line)
                                        if match:
                                            scrub_info = {
                                                'progress': float(match.group(1)),
                                                'time_remaining': match.group(2) if len(match.groups()) > 1 else 'calculating'
                                            }
                        
                        pools[pool_name] = {
                            'size': parts[1],
                            'alloc': parts[2],
                            'free': parts[3],
                            'capacity': int(parts[4].rstrip('%')),
                            'health': parts[5],
                            'resilver': resilver_info,
                            'scrub': scrub_info
                        }
        except Exception as e:
            print(f"Error getting ZFS stats: {e}")
        
        return pools
    
    async def get_docker_stats(self) -> Dict:
        """Get Docker container statistics"""
        stats = {'containers': 0, 'running': 0}
        
        try:
            result = subprocess.run(
                ['docker', 'ps', '-a', '--format', '{{.State}}'],
                capture_output=True, text=True, timeout=5
            )
            
            if result.returncode == 0:
                states = result.stdout.strip().split('\n')
                stats['containers'] = len([s for s in states if s])
                stats['running'] = len([s for s in states if s == 'running'])
        except Exception as e:
            print(f"Error getting Docker stats: {e}")
        
        return stats
    
    async def get_network_stats(self) -> Dict:
        """Get network interface statistics"""
        interfaces = {}
        
        net_io = psutil.net_io_counters(pernic=True)
        for iface, stats in net_io.items():
            if iface != 'lo':  # Skip loopback
                interfaces[iface] = {
                    'bytes_sent': stats.bytes_sent,
                    'bytes_recv': stats.bytes_recv,
                    'packets_sent': stats.packets_sent,
                    'packets_recv': stats.packets_recv
                }
        
        return {'interfaces': interfaces}
    
    async def update_all_stats(self):
        """Update all system statistics"""
        system_state['cpu'] = await self.get_cpu_stats()
        system_state['memory'] = await self.get_memory_stats()
        system_state['disks'] = await self.get_disk_stats()
        system_state['zfs_pools'] = await self.get_zfs_pool_stats()
        system_state['docker'] = await self.get_docker_stats()
        system_state['network'] = await self.get_network_stats()
        
        return system_state
    
    async def detect_hardware_changes(self) -> Optional[Dict]:
        """Detect hardware changes (new disks, removed disks)"""
        current_disks = await self.get_disk_stats()
        
        changes = []
        
        # Check for new disks
        for disk, info in current_disks.items():
            if disk not in self.last_disk_state:
                changes.append({
                    'type': 'disk_added',
                    'disk': disk,
                    'model': info['model'],
                    'size': info['size']
                })
        
        # Check for removed disks
        for disk in self.last_disk_state:
            if disk not in current_disks:
                changes.append({
                    'type': 'disk_removed',
                    'disk': disk
                })
        
        # Check for disk temperature alerts
        for disk, info in current_disks.items():
            if info['temperature'] and info['temperature'] > 50:
                old_temp = self.last_disk_state.get(disk, {}).get('temperature', 0)
                if not old_temp or info['temperature'] > old_temp + 5:
                    changes.append({
                        'type': 'disk_temperature_warning',
                        'disk': disk,
                        'temperature': info['temperature']
                    })
        
        self.last_disk_state = current_disks
        
        return changes if changes else None
    
    async def detect_pool_changes(self) -> Optional[Dict]:
        """Detect ZFS pool changes (resilver started, completed, errors)"""
        current_pools = await self.get_zfs_pool_stats()
        
        changes = []
        
        for pool, info in current_pools.items():
            old_info = self.last_pool_state.get(pool, {})
            
            # Check for resilver start
            if info.get('resilver') and not old_info.get('resilver'):
                changes.append({
                    'type': 'resilver_started',
                    'pool': pool,
                    'progress': info['resilver']['progress']
                })
            
            # Check for resilver completion
            if not info.get('resilver') and old_info.get('resilver'):
                changes.append({
                    'type': 'resilver_completed',
                    'pool': pool
                })
            
            # Check for health changes
            if info['health'] != old_info.get('health', 'ONLINE'):
                changes.append({
                    'type': 'pool_health_change',
                    'pool': pool,
                    'old_health': old_info.get('health', 'UNKNOWN'),
                    'new_health': info['health']
                })
        
        self.last_pool_state = current_pools
        
        return changes if changes else None


class UdevMonitor:
    """Monitors udev events for hardware changes"""
    
    def __init__(self):
        self.context = pyudev.Context()
        self.monitor = pyudev.Monitor.from_netlink(self.context)
        self.monitor.filter_by(subsystem='block')
    
    async def watch_events(self):
        """Watch for udev events"""
        loop = asyncio.get_event_loop()
        
        def handle_event(action, device):
            if action in ('add', 'remove'):
                asyncio.create_task(broadcast_event({
                    'type': 'hardware_event',
                    'action': action,
                    'device': device.device_node,
                    'device_type': device.device_type,
                    'timestamp': datetime.now().isoformat()
                }))
        
        self.monitor.start()
        
        # Poll for events
        while True:
            try:
                device = self.monitor.poll(timeout=1)
                if device:
                    handle_event(device.action, device)
            except Exception as e:
                print(f"Udev monitoring error: {e}")
            
            await asyncio.sleep(0.1)


# WebSocket handlers
async def register_client(websocket):
    """Register a new WebSocket client"""
    clients.add(websocket)
    print(f"Client connected. Total clients: {len(clients)}")
    
    # Send current state immediately
    await websocket.send(json.dumps({
        'type': 'initial_state',
        'data': system_state
    }))


async def unregister_client(websocket):
    """Unregister a WebSocket client"""
    clients.remove(websocket)
    print(f"Client disconnected. Total clients: {len(clients)}")


async def broadcast_event(event: Dict):
    """Broadcast event to all connected clients"""
    if clients:
        message = json.dumps(event)
        await asyncio.gather(
            *[client.send(message) for client in clients],
            return_exceptions=True
        )


async def handle_client(websocket, path):
    """Handle WebSocket client connection"""
    
    # Authenticate client with session token
    try:
        auth_message = await asyncio.wait_for(websocket.recv(), timeout=5.0)
        auth_data = json.loads(auth_message)
        
        session_id = auth_data.get('session_id')
        if not session_id or not validate_session(session_id):
            await websocket.send(json.dumps({
                'type': 'error',
                'message': 'Authentication failed - invalid or expired session'
            }))
            await websocket.close()
            return
        
        print(f"✓ Client authenticated: {session_id[:16]}...")
    except asyncio.TimeoutError:
        print("✗ Client authentication timeout")
        await websocket.close()
        return
    except json.JSONDecodeError:
        print("✗ Invalid authentication message")
        await websocket.close()
        return
    except Exception as e:
        print(f"✗ Authentication error: {e}")
        await websocket.close()
        return
    
    await register_client(websocket)
    
    try:
        async for message in websocket:
            # Handle client messages (ping/pong, requests)
            try:
                data = json.loads(message)
                
                if data.get('type') == 'ping':
                    await websocket.send(json.dumps({'type': 'pong'}))
                elif data.get('type') == 'request_state':
                    await websocket.send(json.dumps({
                        'type': 'state_update',
                        'data': system_state
                    }))
            except json.JSONDecodeError:
                pass
    except websockets.exceptions.ConnectionClosed:
        pass
    finally:
        await unregister_client(websocket)


async def system_monitor_loop():
    """Main monitoring loop"""
    monitor = SystemMonitor()
    
    while True:
        try:
            # Update all stats
            state = await monitor.update_all_stats()
            
            # Broadcast state update
            await broadcast_event({
                'type': 'state_update',
                'data': state,
                'timestamp': datetime.now().isoformat()
            })
            
            # Check for hardware changes
            hw_changes = await monitor.detect_hardware_changes()
            if hw_changes:
                for change in hw_changes:
                    await broadcast_event(change)
                    
                    # Send alerts
                    if alerting:
                        if change['type'] == 'disk_added':
                            alert = AlertTemplates.disk_added(
                                change['disk'], 
                                change['model'], 
                                change['size']
                            )
                            alerting.alert('disk_added', alert['title'], alert['message'], alert['priority'])
                        
                        elif change['type'] == 'disk_removed':
                            alert = AlertTemplates.disk_removed(change['disk'])
                            alerting.alert('disk_removed', alert['title'], alert['message'], alert['priority'])
                        
                        elif change['type'] == 'disk_temperature_warning':
                            alert = AlertTemplates.disk_temp_warning(
                                change['disk'], 
                                change['temperature']
                            )
                            alerting.alert('disk_temp_warning', alert['title'], alert['message'], alert['priority'])
            
            # Check for pool changes
            pool_changes = await monitor.detect_pool_changes()
            if pool_changes:
                for change in pool_changes:
                    await broadcast_event(change)
                    
                    # Send alerts
                    if alerting:
                        if change['type'] == 'pool_degraded' or change['type'] == 'pool_health_change':
                            alert = AlertTemplates.pool_degraded(
                                change['pool'], 
                                change.get('new_health', 'UNKNOWN')
                            )
                            alerting.alert('pool_degraded', alert['title'], alert['message'], alert['priority'])
                        
                        elif change['type'] == 'resilver_started':
                            alert = AlertTemplates.resilver_started(change['pool'])
                            alerting.alert('resilver_started', alert['title'], alert['message'], alert['priority'])
                        
                        elif change['type'] == 'resilver_completed':
                            alert = AlertTemplates.resilver_completed(change['pool'])
                            alerting.alert('resilver_completed', alert['title'], alert['message'], alert['priority'])
            
        except Exception as e:
            print(f"Error in monitor loop: {e}")
        
        await asyncio.sleep(2)  # Update every 2 seconds


async def main():
    """Main entry point"""
    print("="*60)
    print("D-PlaneOS Real-time Event Daemon v2.9.0")
    print("="*60)
    print(f"WebSocket server starting on {WS_HOST}:{WS_PORT}")
    print("Monitoring: CPU, Memory, Disks, ZFS, Docker, Network, Udev")
    print("="*60)
    
    # Start WebSocket server
    ws_server = websockets.serve(handle_client, WS_HOST, WS_PORT)
    
    # Start monitoring tasks
    monitor_task = asyncio.create_task(system_monitor_loop())
    
    # Start udev monitoring
    try:
        udev_monitor = UdevMonitor()
        udev_task = asyncio.create_task(udev_monitor.watch_events())
    except Exception as e:
        print(f"Warning: Could not start udev monitoring: {e}")
        udev_task = None
    
    # Run forever
    if udev_task:
        await asyncio.gather(ws_server, monitor_task, udev_task)
    else:
        await asyncio.gather(ws_server, monitor_task)


if __name__ == '__main__':
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\nShutting down gracefully...")
