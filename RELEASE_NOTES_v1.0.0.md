# Release Notes v1.0.0 - Network Connection Exporter

## Initial Release

### 🚀 Features
- **Network connection monitoring** via `/proc/net/tcp` parsing
- **IPv4 support** with proper address formatting (little-endian to big-endian conversion)
- **Network interface detection** to identify traffic source interfaces  
- **Prometheus metrics export** on port 9100
- **Systemd service integration** with security hardening
- **Multi-host deployment** support with installation scripts

### 📊 Metrics Exported
- `network_connections_total` - Total number of network connections by interface and state
- Labels: `src_addr`, `src_port`, `dst_addr`, `dst_port`, `interface`, `state`

### 🛠 Installation Options

#### Option 1: Download Binary
```bash
# Download the binary
wget https://github.com/bjin01/conn-exporter/releases/download/v1.0.0/conn-exporter
chmod +x conn-exporter

# Run directly
./conn-exporter

# Metrics available at: http://localhost:9100/metrics
```

#### Option 2: Automated Installation (Recommended)
```bash
# Download and run installation script
wget https://raw.githubusercontent.com/bjin01/conn-exporter/main/install-conn-exporter.sh
chmod +x install-conn-exporter.sh
sudo ./install-conn-exporter.sh

# Service will be installed and started automatically
systemctl status conn-exporter
```

#### Option 3: Multi-Host Deployment
```bash
# For deploying to multiple hosts
wget https://raw.githubusercontent.com/bjin01/conn-exporter/main/deploy-to-multiple-hosts.sh
# Edit the hosts array in the script, then run:
./deploy-to-multiple-hosts.sh
```

### 🔧 Configuration

#### Prometheus Scrape Config
```yaml
scrape_configs:
  - job_name: 'conn-exporter'
    static_configs:
      - targets: ['localhost:9100']
        labels:
          host: 'server-01'
```

#### Grafana Dashboard
Import the provided dashboard from: `grafana-dashboard-multi-host.json`

### 📋 Requirements
- **Operating System**: Linux x86_64
- **Go Version**: 1.21+ (for compilation)
- **Dependencies**: Prometheus client library v1.19.1
- **Permissions**: Read access to `/proc/net/tcp` and `/proc/net/dev`

### 🔒 Security Features
- Runs as `nobody` user when installed as systemd service
- No network write capabilities
- Restricted file system access
- Memory and process limits

### 🏗 Architecture
```
conn-exporter
├── TCP connection parsing (/proc/net/tcp)
├── Interface mapping (/proc/net/dev)
├── Prometheus metrics server (port 9100)
└── Systemd service integration
```

### 📈 Monitoring Stack Integration
- **Prometheus**: Ready-to-use scrape configurations
- **Grafana**: Interactive dashboards with host filtering
- **Multi-host**: Centralized monitoring with per-host labels

### 🐛 Known Limitations
- IPv6 support is commented out (ready for future activation)
- Requires Linux `/proc` filesystem
- x86_64 architecture only

### 📚 Documentation
Complete documentation available in the repository:
- `README.md` - Quick start guide
- `multi-host-deployment-guide.md` - Multi-host setup
- `prometheus-scrape-config.yml` - Prometheus configuration
- `grafana-dashboard-multi-host.json` - Grafana dashboard

### 🔄 Next Steps
After installation, verify the service:
```bash
# Check service status
systemctl status conn-exporter

# View metrics
curl http://localhost:9100/metrics | grep network_connections

# Check logs
journalctl -u conn-exporter -f
```

---

**Binary Details:**
- File: `conn-exporter`
- Size: ~12MB
- Architecture: Linux x86_64
- Build: Static binary with debug symbols