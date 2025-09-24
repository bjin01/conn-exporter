# Grafana Dashboard Host Selection Fix Guide - UPDATED

## Problem
The Grafana dashboard shows no hosts in the dropdown because:
1. The dashboard expects `host` labels in metrics
2. Your current Prometheus config only provides `instance` labels  
3. The `instance` label contains `hostname:port` (e.g., `sap03.susedemo.de:9100`)

## Solution: Add Metric Relabeling to Prometheus

### Step 1: Update prometheus.yml

Add `metric_relabel_configs` to your conn-exporter job:

```yaml
scrape_configs:
  - job_name: 'conn-exporter-multi-host'
    scrape_interval: 15s
    scrape_timeout: 10s
    metrics_path: /metrics
    scheme: http

    static_configs:
      - targets:
          - 'sap03.susedemo.de:9100'
          - 'bokvm.susedemo.de:9100'
        labels:
          environment: 'production'
          datacenter: 'dc1'

    # ADD THIS SECTION:
    metric_relabel_configs:
      # Extract full hostname (remove :9100 port)
      - source_labels: [__address__]
        regex: '([^:]+):.*'
        target_label: hostname
        replacement: '${1}'
      
      # Extract short hostname for dashboard (sap03, bokvm) 
      - source_labels: [hostname]
        regex: '([^.]+)\\..*'
        target_label: host
        replacement: '${1}'
```

### Step 2: Restart Prometheus

```bash
sudo systemctl restart prometheus
# or reload configuration
sudo kill -HUP $(pgrep prometheus)
```

### Step 3: Verify Labels Are Created

Test that the new labels exist:

```bash
# Run the compatibility test script
./test-dashboard-compatibility.sh

# Or check manually
curl -s "http://localhost:9090/api/v1/label/host/values" | jq '.data'
```

Expected output:
```json
["bokvm", "sap03"]
```

### Step 4: Import Updated Dashboard

The updated `grafana-dashboard-multi-host.json` will now work with:
- **Host dropdown**: Shows `sap03`, `bokvm` (short names)
- **Interface filtering**: Works per selected host  
- **State filtering**: Works per selected host

## Alternative Quick Fix (If You Can't Change Prometheus Config)

If you cannot modify prometheus.yml, use this alternative dashboard configuration:

1. In Grafana, edit the dashboard variables:
2. Change the Host variable query from:
   ```
   label_values(network_connections_info, host)
   ```
   To:
   ```
   label_values(network_connections_info, instance)
   ```

3. Update the variable regex to extract hostname:
   ```
   Regex: /([^.]+)\\..*:9100/
   ```

This will show full hostnames in the dropdown but requires manual editing.

## Verification

After implementing the fix:

1. **Prometheus targets**: All should show as UP
2. **Dashboard variables**: Host dropdown should populate  
3. **Panels**: Should show data when hosts are selected
4. **Queries**: Should filter correctly by host selection

Run `./test-dashboard-compatibility.sh` to verify everything works.

## Troubleshooting

### No hosts in dropdown:
- Check Prometheus metric relabeling is configured
- Verify Prometheus has restarted/reloaded  
- Check that conn-exporter metrics have `host` label

### Dashboard shows no data:
- Verify Grafana datasource points to correct Prometheus
- Check time range in dashboard
- Ensure conn-exporter is running on target hosts

### Variables not filtering:
- Check that panel queries use `{host=~"$host"}` syntax
- Verify variable dependencies (interface depends on host)
- Check for syntax errors in PromQL queries

## Complete Configuration Files

Use these ready-to-deploy configuration files:

### prometheus-enhanced.yml
Complete Prometheus configuration with metric relabeling

### grafana-dashboard-multi-host.json  
Updated dashboard with correct host variable mapping

### test-dashboard-compatibility.sh
Script to validate the monitoring stack setup

All files have been updated to work together for proper multi-host monitoring.
