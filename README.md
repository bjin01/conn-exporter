# Network Connection Exporter (conn-exporter)

A Prometheus exporter that monitors network connections on Linux systems by reading from `/proc/net/tcp` and `/proc/net/tcp6`. It provides detailed metrics about active network connections including source/destination addresses, ports, connection states, and network interfaces.

## Features

- ✅ **IPv4 Connection Monitoring** - Tracks TCP connections from `/proc/net/tcp`
- ✅ **Interface Detection** - Automatically maps connections to network interfaces (eth0, lo, etc.)
- ✅ **Connection State Tracking** - Monitors ESTABLISHED, LISTEN, TIME_WAIT, and other states
- ✅ **Prometheus Metrics** - Exports metrics in Prometheus format
- ✅ **Multi-Host Support** - Ready for deployment across multiple servers
- ✅ **IPv6 Ready** - IPv6 support available (currently commented out)

## Quick Start

### Build the exporter
```bash
go build -o conn-exporter .
```

### Run locally
```bash
./conn-exporter
```

### Install as systemd service
```bash
sudo ./install-conn-exporter.sh
```

The exporter will start serving metrics on port `9100` at `/metrics` endpoint.

## Metrics

The exporter provides the following metric:

```
network_connections_info{source_address, source_port, destination_address, destination_port, state, interface} 1
```

### Example metrics output:
```
network_connections_info{destination_address="0.0.0.0",destination_port="0",interface="lo",source_address="127.0.0.1",source_port="22",state="LISTEN"} 1
network_connections_info{destination_address="192.168.1.100",destination_port="443",interface="eth0",source_address="192.168.1.10",source_port="54321",state="ESTABLISHED"} 1
```

## Installation

### Single Host Installation

1. Build the binary:
   ```bash
   go build -o conn-exporter .
   ```

2. Install as systemd service:
   ```bash
   sudo ./install-conn-exporter.sh
   ```

3. Verify installation:
   ```bash
   curl http://localhost:9100/metrics | grep network_connections_info
   ```

### Multi-Host Deployment

For deploying to multiple hosts, use the provided deployment script:

```bash
# Edit the script to configure your hosts
nano deploy-to-multiple-hosts.sh

# Deploy to all configured hosts
./deploy-to-multiple-hosts.sh
```

## Prometheus Configuration

Add this to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'conn-exporter'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:9100']
        labels:
          service: 'network-monitoring'
```

For multi-host setup, see `prometheus-multi-host-config.yml` for advanced configuration examples.

## Grafana Dashboard

Import the provided Grafana dashboards:

- **Single Host**: `grafana-dashboard-fixed.json`
- **Multi-Host**: `grafana-dashboard-multi-host.json`

The multi-host dashboard includes:
- Host selection dropdown
- Interface and state filtering
- Connection overview tables
- Time series graphs
- Top ports analysis

## Project Structure

```
conn-exporter/
├── main.go                              # Main exporter code
├── go.mod                              # Go module file
├── install-conn-exporter.sh            # Single host installation script
├── deploy-to-multiple-hosts.sh         # Multi-host deployment script
├── prometheus-scrape-config.yml        # Basic Prometheus config
├── prometheus-multi-host-config.yml    # Advanced multi-host config
├── grafana-dashboard-fixed.json        # Single host Grafana dashboard
├── grafana-dashboard-multi-host.json   # Multi-host Grafana dashboard
├── promql-queries.txt                  # Useful PromQL queries
├── multi-host-deployment-guide.md      # Detailed deployment guide
└── README.md                           # This file
```

## Configuration

### Connection States

The exporter maps TCP connection states from `/proc/net/tcp`:

- `01` → `ESTABLISHED`
- `02` → `SYN_SENT`
- `03` → `SYN_RECV`
- `04` → `FIN_WAIT1`
- `05` → `FIN_WAIT2`
- `06` → `TIME_WAIT`
- `07` → `CLOSE`
- `08` → `CLOSE_WAIT`
- `09` → `LAST_ACK`
- `0A` → `LISTEN`
- `0B` → `CLOSING`

### Interface Detection

The exporter automatically detects network interfaces by:
1. Reading all available network interfaces
2. Mapping IP addresses to their corresponding interfaces
3. Providing fallback labels for special addresses (0.0.0.0 → lo, 127.0.0.1 → lo)

## Useful PromQL Queries

```promql
# Total connections
sum(network_connections_info)

# Connections by state
sum(network_connections_info) by (state)

# Connections by interface
sum(network_connections_info) by (interface)

# Established connections only
sum(network_connections_info{state="ESTABLISHED"})

# External connections (non-localhost)
sum(network_connections_info{source_address!~"127\\.0\\.0\\.1|0\\.0\\.0\\.0"})

# Top 10 destination ports
topk(10, sum(network_connections_info) by (destination_port))
```

See `promql-queries.txt` for more examples.

## Requirements

- Go 1.21 or later
- Linux system with `/proc/net/tcp` access
- Prometheus (for metrics collection)
- Grafana (for visualization)

## Security

The systemd service runs with security hardening:
- Runs as `nobody:nogroup` user
- Protected system and home directories
- Restricted system calls and capabilities
- No new privileges allowed

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For questions and support:
- Create an issue in this repository
- Check the troubleshooting section in `multi-host-deployment-guide.md`

## Monitoring Best Practices

### Alerting Rules

```yaml
# Example Prometheus alerting rules
groups:
  - name: conn-exporter
    rules:
      - alert: TooManyConnections
        expr: sum(network_connections_info) > 10000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Too many network connections detected"
          
      - alert: NoListeningSSH
        expr: absent(network_connections_info{state="LISTEN", source_port="22"})
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "SSH service not listening"
```

### Performance Considerations

- Default scrape interval: 15 seconds
- Memory usage: ~10-20MB per instance
- CPU usage: Minimal (<1% on typical systems)
- Network overhead: ~1-5KB per scrape depending on connection count