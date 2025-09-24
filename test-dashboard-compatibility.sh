#!/bin/bash

# Test script to verify Grafana dashboard variables will work
# This checks if the required labels are present in the metrics

echo "=== Testing Grafana Dashboard Compatibility ==="
echo ""

# Test if Prometheus is running and accessible
echo "1. Testing Prometheus connectivity..."
if curl -s http://localhost:9090/api/v1/label/__name__/values | grep -q "network_connections_info"; then
    echo "✓ Prometheus is accessible and conn-exporter metrics are present"
else
    echo "✗ Prometheus not accessible or no conn-exporter metrics found"
    echo "Make sure Prometheus is running and configured to scrape conn-exporter"
    exit 1
fi

echo ""

# Test for host labels
echo "2. Testing for 'host' labels (required for dashboard)..."
host_labels=$(curl -s "http://localhost:9090/api/v1/label/host/values" 2>/dev/null | jq -r '.data[]?' 2>/dev/null || echo "")

if [ -n "$host_labels" ]; then
    echo "✓ Host labels found:"
    echo "$host_labels" | sed 's/^/    /'
else
    echo "✗ No 'host' labels found"
    echo ""
    echo "Available hosts from 'instance' label:"
    curl -s "http://localhost:9090/api/v1/label/instance/values" 2>/dev/null | jq -r '.data[]?' 2>/dev/null | sed 's/^/    /' || echo "    None found"
    echo ""
    echo "You need to add metric relabeling to your prometheus.yml:"
    echo "    metric_relabel_configs:"
    echo "      - source_labels: [__address__]"
    echo "        regex: '([^:]+):.*'"  
    echo "        target_label: hostname"
    echo "        replacement: '\${1}'"
    echo "      - source_labels: [hostname]"
    echo "        regex: '([^.]+)\\..*'"
    echo "        target_label: host" 
    echo "        replacement: '\${1}'"
fi

echo ""

# Test for interface labels  
echo "3. Testing for 'interface' labels..."
interface_count=$(curl -s "http://localhost:9090/api/v1/label/interface/values" 2>/dev/null | jq -r '.data | length' 2>/dev/null || echo "0")

if [ "$interface_count" -gt 0 ]; then
    echo "✓ Interface labels found ($interface_count interfaces)"
    curl -s "http://localhost:9090/api/v1/label/interface/values" 2>/dev/null | jq -r '.data[]?' 2>/dev/null | sed 's/^/    /' || true
else
    echo "✗ No interface labels found"
fi

echo ""

# Test for state labels
echo "4. Testing for 'state' labels..."  
state_count=$(curl -s "http://localhost:9090/api/v1/label/state/values" 2>/dev/null | jq -r '.data | length' 2>/dev/null || echo "0")

if [ "$state_count" -gt 0 ]; then
    echo "✓ State labels found ($state_count states)"
    curl -s "http://localhost:9090/api/v1/label/state/values" 2>/dev/null | jq -r '.data[]?' 2>/dev/null | sed 's/^/    /' || true
else
    echo "✗ No state labels found"
fi

echo ""

# Sample query test
echo "5. Testing sample dashboard query..."
sample_query="sum(network_connections_info) by (host)"
result=$(curl -s -G "http://localhost:9090/api/v1/query" --data-urlencode "query=$sample_query" 2>/dev/null)

if echo "$result" | jq -e '.status == "success"' >/dev/null 2>&1; then
    echo "✓ Sample query works:"
    echo "   Query: $sample_query"
    
    # Show results
    echo "$result" | jq -r '.data.result[]? | "    " + .metric.host + ": " + .value[1] + " connections"' 2>/dev/null || echo "    No results or parse error"
else
    echo "✗ Sample query failed"
    echo "   Query: $sample_query"
    echo "   This query is used by the dashboard - it needs to work for host selection"
fi

echo ""
echo "=== Summary ==="
if [ -n "$host_labels" ] && [ "$interface_count" -gt 0 ] && [ "$state_count" -gt 0 ]; then
    echo "✓ Dashboard should work! All required labels are present."
    echo ""
    echo "Next steps:"
    echo "1. Import grafana-dashboard-multi-host.json into Grafana"
    echo "2. Configure Grafana data source to point to your Prometheus"
    echo "3. The host dropdown should now show: $host_labels"
else
    echo "✗ Dashboard will have issues. Missing required labels."
    echo ""
    echo "Fix by updating your prometheus.yml with the metric_relabel_configs shown above,"
    echo "then restart Prometheus and run this test again."
fi
