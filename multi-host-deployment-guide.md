# Multi-Host conn-exporter Deployment Guide

## Overview
This guide helps you deploy conn-exporter on multiple hosts and configure centralized monitoring with Prometheus and Grafana.

## 📋 Prerequisites
- Multiple Linux hosts where you want to monitor network connections
- Central Prometheus server
- Grafana instance
- SSH access to all target hosts

## 🚀 Deployment Steps

### Step 1: Deploy conn-exporter to Multiple Hosts

#### Option A: Manual Deployment Script
```bash
#!/bin/bash
# deploy-conn-exporter.sh

HOSTS=(
    "user@server1.example.com"
    "user@server2.example.com" 
    "user@server3.example.com"
    "user@dev-server1.example.com"
)

CONN_EXPORTER_BINARY="./conn-exporter"

for host in "${HOSTS[@]}"; do
    echo "Deploying to $host..."
    
    # Copy binary
    scp $CONN_EXPORTER_BINARY $host:/tmp/
    
    # Install and setup service
    ssh $host << 'EOF'
        # Create user and directories
        sudo useradd --system --shell /bin/false conn-exporter || true
        sudo mkdir -p /opt/conn-exporter
        sudo mv /tmp/conn-exporter /opt/conn-exporter/
        sudo chown -R conn-exporter:conn-exporter /opt/conn-exporter
        sudo chmod +x /opt/conn-exporter/conn-exporter
        
        # Create systemd service
        sudo tee /etc/systemd/system/conn-exporter.service > /dev/null << 'EOL'
[Unit]
Description=Network Connection Exporter
After=network.target

[Service]
Type=simple
User=conn-exporter
Group=conn-exporter
ExecStart=/opt/conn-exporter/conn-exporter
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOL
        
        # Enable and start service
        sudo systemctl daemon-reload
        sudo systemctl enable conn-exporter
        sudo systemctl start conn-exporter
        
        # Check status
        sudo systemctl status conn-exporter --no-pager
EOF
    
    echo "Deployment to $host completed"
    echo "---"
done
```

#### Option B: Ansible Playbook
```yaml
# deploy-conn-exporter.yml
---
- name: Deploy conn-exporter to multiple hosts
  hosts: all
  become: yes
  vars:
    conn_exporter_user: conn-exporter
    conn_exporter_binary: ./conn-exporter
    conn_exporter_port: 9100
    
  tasks:
    - name: Create conn-exporter user
      user:
        name: "{{ conn_exporter_user }}"
        system: yes
        shell: /bin/false
        home: /opt/conn-exporter
        create_home: yes
        
    - name: Copy conn-exporter binary
      copy:
        src: "{{ conn_exporter_binary }}"
        dest: /opt/conn-exporter/conn-exporter
        owner: "{{ conn_exporter_user }}"
        group: "{{ conn_exporter_user }}"
        mode: '0755'
        
    - name: Create systemd service file
      copy:
        content: |
          [Unit]
          Description=Network Connection Exporter
          After=network.target
          
          [Service]
          Type=simple
          User={{ conn_exporter_user }}
          Group={{ conn_exporter_user }}
          ExecStart=/opt/conn-exporter/conn-exporter
          Restart=always
          RestartSec=5
          
          [Install]
          WantedBy=multi-user.target
        dest: /etc/systemd/system/conn-exporter.service
        
    - name: Reload systemd
      systemd:
        daemon_reload: yes
        
    - name: Enable and start conn-exporter
      systemd:
        name: conn-exporter
        enabled: yes
        state: started
        
    - name: Open firewall port (if using firewalld)
      firewalld:
        port: "{{ conn_exporter_port }}/tcp"
        permanent: yes
        state: enabled
        immediate: yes
      ignore_errors: yes
      
    - name: Verify service is running
      uri:
        url: "http://localhost:{{ conn_exporter_port }}/metrics"
        method: GET
      delegate_to: "{{ inventory_hostname }}"
```

### Step 2: Configure Prometheus

Add to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'conn-exporter-production'
    scrape_interval: 15s
    static_configs:
      - targets:
          - 'web-server-01:9100'
          - 'web-server-02:9100'
          - 'db-server-01:9100'
          - 'app-server-01:9100'
        labels:
          environment: 'production'
          datacenter: 'dc1'
          
  - job_name: 'conn-exporter-development'
    scrape_interval: 15s
    static_configs:
      - targets:
          - 'dev-server-01:9100'
          - 'dev-server-02:9100'
        labels:
          environment: 'development'
          datacenter: 'dc1'
    
    # Extract hostname from target address
    relabel_configs:
      - source_labels: [__address__]
        target_label: __tmp_host
        regex: '([^:]+)(:\d+)?'
        replacement: '${1}'
      - source_labels: [__tmp_host]
        target_label: host
        regex: '([^.]+)\..*'
        replacement: '${1}'
      - source_labels: [__tmp_host]
        target_label: hostname
```

### Step 3: Import Grafana Dashboard

1. Import `grafana-dashboard-multi-host.json`
2. Select your Prometheus data source
3. The dashboard will show dropdowns for:
   - **Host** - Select specific hosts or "All"
   - **Interface** - Filter by network interface
   - **State** - Filter by connection state
   - **Environment** - Filter by environment label

## 🔧 Configuration Examples

### File-based Service Discovery
Create `/etc/prometheus/targets/conn-exporter-prod.yml`:
```yaml
- targets:
    - 'web-server-01.prod.example.com:9100'
    - 'web-server-02.prod.example.com:9100'
  labels:
    environment: 'production'
    role: 'webserver'
    datacenter: 'dc1'

- targets:
    - 'db-server-01.prod.example.com:9100'
  labels:
    environment: 'production'
    role: 'database'
    datacenter: 'dc1'
```

### Docker Deployment
```yaml
# docker-compose.yml
version: '3.8'
services:
  conn-exporter:
    build: .
    ports:
      - "9100:9100"
    restart: unless-stopped
    network_mode: "host"  # Required to access host network connections
    labels:
      - "prometheus.io/scrape=true"
      - "prometheus.io/port=9100"
      - "prometheus.io/job=conn-exporter"
```

## 🔍 Verification

### Check Individual Hosts
```bash
# Test each host
for host in server1 server2 server3; do
    echo "Checking $host..."
    curl -s http://$host:9100/metrics | grep network_connections_info | wc -l
done
```

### Verify Prometheus Targets
1. Go to Prometheus UI: `http://prometheus:9090/targets`
2. Check all conn-exporter targets are "UP"
3. Verify labels are correctly applied

### Test Grafana Dashboard
1. Open the multi-host dashboard
2. Verify host dropdown is populated
3. Test filtering by different hosts
4. Confirm data appears for all hosts

## 🚨 Troubleshooting

### Host Not Appearing in Dropdown
- Check Prometheus targets are UP
- Verify `host` label is being applied correctly
- Check Grafana dashboard variable queries

### No Data for Specific Host
```bash
# Check if conn-exporter is running
ssh user@problem-host "sudo systemctl status conn-exporter"

# Test metrics endpoint
curl http://problem-host:9100/metrics

# Check Prometheus logs
grep "problem-host" /var/log/prometheus/prometheus.log
```

### Firewall Issues
```bash
# Open port 9100 on target hosts
sudo firewall-cmd --permanent --add-port=9100/tcp
sudo firewall-cmd --reload

# Or for iptables
sudo iptables -A INPUT -p tcp --dport 9100 -j ACCEPT
```

## 📊 Dashboard Features

The multi-host dashboard includes:

1. **Hosts Overview** - Summary table of all hosts and connection counts
2. **Host Selector** - Dropdown to select specific hosts or view all
3. **Network Connections Table** - Detailed view filtered by selected host(s)
4. **Time Series Graphs** - Connections by state, host, and interface
5. **Top Ports Analysis** - Most active ports across selected hosts
6. **Environment Filtering** - Filter by production/development/staging

## 🔄 Maintenance

### Update conn-exporter on All Hosts
```bash
#!/bin/bash
# update-conn-exporter.sh

HOSTS=("server1" "server2" "server3")
NEW_BINARY="./conn-exporter-new"

for host in "${HOSTS[@]}"; do
    echo "Updating $host..."
    scp $NEW_BINARY $host:/tmp/conn-exporter-new
    ssh $host << 'EOF'
        sudo systemctl stop conn-exporter
        sudo mv /tmp/conn-exporter-new /opt/conn-exporter/conn-exporter
        sudo chown conn-exporter:conn-exporter /opt/conn-exporter/conn-exporter
        sudo chmod +x /opt/conn-exporter/conn-exporter
        sudo systemctl start conn-exporter
        sudo systemctl status conn-exporter --no-pager
EOF
done
```

This setup provides centralized monitoring of network connections across all your hosts with easy drill-down capabilities in Grafana!