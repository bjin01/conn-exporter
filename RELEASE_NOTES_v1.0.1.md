# Release Notes v1.0.1 - Network Connection Exporter (Patch Release)

## 🐛 Bug Fixes

### Fixed Network Interface Detection Errors
- **Resolved**: `route ip+net: netlinkrib: address family not supported by protocol` error
- **Improved**: Graceful handling of complex network interface types
- **Enhanced**: Error recovery and interface cache management

### 🔧 Technical Improvements

#### Interface Detection Robustness
- **Skip problematic interfaces**: Automatically skip down/loopback interfaces that cause protocol errors
- **Better error handling**: Continue processing other interfaces when one fails
- **Dynamic interface support**: Handle veth (container), bonding, teaming, and other complex interface types
- **Cache refresh logic**: Automatically refresh interface mappings when IP addresses change

#### Error Handling Enhancements  
- **Graceful degradation**: Service continues running even when some interfaces can't be read
- **Detailed logging**: Better error messages for troubleshooting network configuration issues
- **Fallback mechanisms**: Use "unknown" interface name instead of crashing

### 🌐 Supported Network Configurations
This release specifically improves compatibility with:
- **Container environments** (Docker, Podman with veth interfaces)
- **Bonded/teamed interfaces** (network interface bonding)
- **Complex routing setups** (multiple interface types)
- **Dynamic networks** (interfaces that come and go)

### 📦 Installation

#### Quick Fix for Existing v1.0.0 Users
```bash
# Stop existing service
sudo systemctl stop conn-exporter

# Download v1.0.1 binary  
wget https://github.com/bjin01/conn-exporter/releases/download/v1.0.1/conn-exporter
sudo cp conn-exporter /usr/bin/conn-exporter
sudo chmod +x /usr/bin/conn-exporter

# Restart service
sudo systemctl start conn-exporter
sudo systemctl status conn-exporter
```

#### Fresh Installation
```bash
# Download latest installation script
wget https://raw.githubusercontent.com/bjin01/conn-exporter/main/install-conn-exporter.sh
chmod +x install-conn-exporter.sh
sudo ./install-conn-exporter.sh
```

### 🔍 Verification

After upgrading, verify the fix worked:
```bash
# Check service status (should be active)
systemctl status conn-exporter

# Check logs (should not show interface errors)
journalctl -u conn-exporter --since "1 minute ago"

# Test metrics endpoint
curl http://localhost:9100/metrics | grep network_connections
```

### 📈 What's Fixed

**Before v1.0.1:**
```
Error getting network interfaces: route ip+net: netlinkrib: address family not supported by protocol
```

**After v1.0.1:**
```
2025/09/20 12:55:55 Beginning to serve on port :9100
# Clean startup with metrics working properly
```

### 🎯 Changes in Detail

#### Code Changes
- Enhanced `getNetworkInterfaces()` function with better error handling
- Added interface filtering (skip down/loopback interfaces)
- Implemented interface cache refresh mechanism  
- Improved IP address type checking and validation

#### Compatibility
- **Maintains**: Full backward compatibility with v1.0.0 configurations
- **Preserves**: All existing metrics and label structures
- **No changes**: Prometheus scrape configs or Grafana dashboards

### 🔄 Migration Notes

**No configuration changes required** - this is a drop-in replacement for v1.0.0.

All existing:
- Prometheus configurations
- Grafana dashboards  
- Systemd service files
- Installation scripts

Continue to work without modification.

### 🚀 Next Steps

After installation:
```bash
# Verify interface detection is working
curl -s http://localhost:9100/metrics | grep 'network_connections.*interface=' | head -5

# Check that various interface types are detected
ip addr show | grep -E '^[0-9]+:' | awk '{print $2}' | sed 's/:$//'
```

---

**Binary Details:**
- File: `conn-exporter`
- Size: ~12MB  
- Architecture: Linux x86_64
- Go Version: 1.21+
- **Fixed**: Network interface detection errors

**Compatibility:**
- Drop-in replacement for v1.0.0
- No configuration changes needed
- Same metrics schema and endpoints