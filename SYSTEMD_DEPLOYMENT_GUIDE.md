# Systemd Deployment Guide for conn-exporter

## Issue Resolution

The conn-exporter failed when running as a systemd service due to several security and permission restrictions. This has been resolved with the following fixes:

### Root Cause Analysis

1. **User Permissions**: Original service ran as `nobody` user, which lacked permissions to:
   - Read network information from `/proc/net/`
   - Execute `ip` commands for interface detection
   - Access network-related syscalls

2. **Security Restrictions**: Overly restrictive systemd security settings:
   - `SystemCallFilter=@system-service` blocked network monitoring syscalls
   - `PrivateDevices=true` prevented access to network devices
   - `User=nobody` lacked network monitoring capabilities

3. **Path Issues**: Limited PATH environment in systemd didn't include `ip` command locations

## Solutions Implemented

### 1. Enhanced Binary (`conn-exporter-static`)

- **Multiple IP Path Detection**: Tries `/usr/bin/ip`, `/bin/ip`, `/sbin/ip`, `/usr/sbin/ip`
- **Robust Fallback**: Better error handling when commands fail
- **Debug Logging**: Enhanced logging for troubleshooting systemd environments

### 2. Updated Systemd Service Configuration

```ini
[Unit]
Description=Network Connection Exporter
Documentation=https://github.com/bjin01/conn-exporter
After=network.target
Wants=network.target

[Service]
Type=simple
ExecStart=/usr/bin/conn-exporter
Restart=always
RestartSec=5

# Run as root to access network information
User=root
Group=root

# Environment
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# Security settings (relaxed for network monitoring)
NoNewPrivileges=true
ProtectSystem=true
ProtectHome=true
ProtectKernelTunables=false
ProtectKernelModules=true
ProtectControlGroups=true
PrivateTmp=true
PrivateDevices=false
RestrictAddressFamilies=AF_INET AF_INET6 AF_NETLINK AF_UNIX
RestrictNamespaces=false
RestrictRealtime=true
RestrictSUIDSGID=false
LockPersonality=true
MemoryDenyWriteExecute=false

# Allow network monitoring syscalls
SystemCallFilter=@system-service @network-io @process
SystemCallErrorNumber=EPERM

# Capabilities needed for network monitoring
AmbientCapabilities=CAP_NET_RAW CAP_NET_ADMIN
CapabilityBoundingSet=CAP_NET_RAW CAP_NET_ADMIN

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
```

## Deployment Instructions

### 1. Build and Install Binary

```bash
# Build static binary
cd /path/to/conn-exporter
CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o conn-exporter-static main.go

# Install binary
sudo cp ./conn-exporter-static /usr/bin/conn-exporter
sudo chmod +x /usr/bin/conn-exporter
```

### 2. Install Systemd Service

```bash
# Install service file
sudo cp ./conn-exporter.service /etc/systemd/system/

# Reload systemd and enable service
sudo systemctl daemon-reload
sudo systemctl enable conn-exporter
sudo systemctl start conn-exporter
```

### 3. Verify Operation

```bash
# Check service status
sudo systemctl status conn-exporter

# Check logs
sudo journalctl -u conn-exporter -f

# Test metrics endpoint
curl http://localhost:9100/metrics | grep network_connections_info
```

## Verification Results

✅ **Service Status**: Active (running)  
✅ **Interface Detection**: All interfaces properly mapped  
✅ **Unknown Interfaces**: 0 (previously had many)  
✅ **Metrics Endpoint**: Working correctly on port 9100  

### Interface Mapping Verification

```
Successfully mapped 3 IP addresses to network interfaces
Interface statistics:
  eth0: 1 IP ([192.168.200.179])
  virbr2: 1 IP ([172.16.10.1])  
  virbr1: 1 IP ([192.168.100.1])
```

## Security Considerations

### Production Deployment Options

#### Option 1: Root User (Current - Simple)
- **Pros**: Guaranteed access to all network information
- **Cons**: Runs with elevated privileges
- **Use**: Development, testing, trusted environments

#### Option 2: Dedicated User with Capabilities (Available)
- **File**: `conn-exporter-secure.service`
- **User**: Creates dedicated `conn-exporter` user
- **Capabilities**: Minimal required network monitoring capabilities
- **Use**: Production environments requiring principle of least privilege

### Setup for Secure Option

```bash
# Create dedicated user
sudo useradd -r -s /bin/false conn-exporter

# Install secure service
sudo cp ./conn-exporter-secure.service /etc/systemd/system/conn-exporter.service
sudo systemctl daemon-reload
sudo systemctl restart conn-exporter
```

## Troubleshooting

### Common Issues

1. **Permission Denied**: Ensure proper user permissions or use root user
2. **Command Not Found**: Verify `ip` command paths in system
3. **Interface Detection Fails**: Check logs for fallback detection messages
4. **Port Already in Use**: Ensure no other process is using port 9100

### Debug Commands

```bash
# Check service logs
sudo journalctl -u conn-exporter -n 50

# Test binary directly
sudo /usr/bin/conn-exporter

# Verify network permissions
sudo -u conn-exporter ip addr show  # (if using secure option)
```
