#!/bin/bash

# Prometheus Remote Diagnostics Script
# Run this on your Prometheus server or from a machine that can access it

echo "=== Prometheus Remote Diagnostics ==="
echo

# Replace with your Prometheus server URL
PROMETHEUS_URL="${PROMETHEUS_URL:-http://localhost:9090}"

echo "Testing Prometheus at: $PROMETHEUS_URL"
echo

# 1. Check if Prometheus is accessible
echo "1. Testing Prometheus connectivity..."
if curl -s "$PROMETHEUS_URL/api/v1/targets" > /dev/null 2>&1; then
    echo "✓ Prometheus is accessible"
else
    echo "✗ Cannot reach Prometheus at $PROMETHEUS_URL"
    echo "  Set PROMETHEUS_URL environment variable if different"
    echo "  Example: PROMETHEUS_URL=http://your-prometheus:9090 $0"
    exit 1
fi

echo

# 2. Check targets status
echo "2. Checking conn-exporter targets..."
TARGETS=$(curl -s "$PROMETHEUS_URL/api/v1/targets" | jq -r '.data.activeTargets[] | select(.job == "conn-exporter-multi-host") | "\(.scrapeUrl) - \(.health)"' 2>/dev/null)

if [ -z "$TARGETS" ]; then
    echo "✗ No conn-exporter-multi-host targets found"
    echo "  Check that your prometheus.yml includes the job configuration"
else
    echo "Found targets:"
    echo "$TARGETS"
fi

echo

# 3. Check if conn-exporter metrics exist
echo "3. Testing for conn-exporter metrics..."
METRICS=$(curl -s "$PROMETHEUS_URL/api/v1/query?query=network_connections_info" | jq -r '.data.result | length' 2>/dev/null)

if [ "$METRICS" = "0" ] || [ -z "$METRICS" ]; then
    echo "✗ No network_connections_info metrics found"
    echo "  Make sure conn-exporter is running on target hosts"
    echo "  Check that targets are UP in step 2"
else
    echo "✓ Found $METRICS metric series"
fi

echo

# 4. Check available labels
echo "4. Checking available labels..."

# Check instance labels
echo "Instance labels:"
curl -s "$PROMETHEUS_URL/api/v1/label/instance/values" | jq -r '.data[]' 2>/dev/null | head -5

echo

# Check if host labels exist (what we need for dashboard)
echo "Host labels:"
HOST_LABELS=$(curl -s "$PROMETHEUS_URL/api/v1/label/host/values" | jq -r '.data[]?' 2>/dev/null)

if [ -z "$HOST_LABELS" ]; then
    echo "✗ No 'host' labels found - this is the problem!"
    echo
    echo "SOLUTION NEEDED:"
    echo "Add this to your prometheus.yml under the conn-exporter job:"
    echo
    echo "    metric_relabel_configs:"
    echo "      - source_labels: [__address__]"
    echo "        regex: '([^:]+):.*'"
    echo "        target_label: hostname"
    echo "        replacement: '\${1}'"
    echo "      - source_labels: [hostname]"
    echo "        regex: '([^.]+)\\..*'"
    echo "        target_label: host"
    echo "        replacement: '\${1}'"
    echo
    echo "Then restart Prometheus"
else
    echo "✓ Found host labels:"
    echo "$HOST_LABELS"
fi

echo

# 5. Sample query for dashboard troubleshooting
echo "5. Testing dashboard query..."
QUERY_RESULT=$(curl -s "$PROMETHEUS_URL/api/v1/query?query=label_values(network_connections_info,%20host)" | jq -r '.data.result[].value[1]?' 2>/dev/null)

if [ -z "$QUERY_RESULT" ]; then
    echo "✗ Dashboard query 'label_values(network_connections_info, host)' returns empty"
    echo "  This explains why the dropdown is empty"
else
    echo "✓ Dashboard query works - hosts should appear:"
    echo "$QUERY_RESULT"
fi

echo
echo "=== Next Steps ==="
if [ -z "$HOST_LABELS" ]; then
    echo "1. Update your prometheus.yml with the metric_relabel_configs shown above"
    echo "2. Restart Prometheus: sudo systemctl restart prometheus"
    echo "3. Wait 1-2 minutes for metrics to be re-scraped"
    echo "4. Refresh your Grafana dashboard"
else
    echo "✓ Configuration looks correct"
    echo "Try refreshing the Grafana dashboard or check Grafana's Prometheus datasource connection"
fi
