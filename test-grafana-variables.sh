#!/bin/bash

# Test Grafana Dashboard Variable Queries
# Run this to debug why the host dropdown isn't populating

echo "=== Testing Grafana Dashboard Variable Queries ==="
echo

# Set your Prometheus URL
PROMETHEUS_URL="${PROMETHEUS_URL:-http://localhost:9090}"
echo "Using Prometheus at: $PROMETHEUS_URL"
echo

# Test 1: Check if network_connections_info metrics exist
echo "1. Testing base metric exists..."
METRIC_COUNT=$(curl -s "$PROMETHEUS_URL/api/v1/query?query=network_connections_info" | jq '.data.result | length' 2>/dev/null)
echo "Found $METRIC_COUNT network_connections_info metric series"

if [ "$METRIC_COUNT" = "0" ] || [ -z "$METRIC_COUNT" ]; then
    echo "❌ No network_connections_info metrics found!"
    echo "   Make sure conn-exporter is running and Prometheus is scraping it"
    exit 1
fi

echo

# Test 2: Check what labels are available on the metric
echo "2. Checking available labels on network_connections_info..."
curl -s "$PROMETHEUS_URL/api/v1/series?match[]=network_connections_info" | jq -r '.data[0] // empty' 2>/dev/null

echo

# Test 3: Test the exact query used by Grafana dashboard
echo "3. Testing Grafana variable query: label_values(network_connections_info, host)"
HOST_VALUES=$(curl -s "$PROMETHEUS_URL/api/v1/query?query=label_values(network_connections_info,%20host)" 2>/dev/null)
echo "Raw response:"
echo "$HOST_VALUES" | jq '.' 2>/dev/null

echo

# Test 4: Check label_values API directly
echo "4. Testing label values API for 'host' label..."
LABEL_VALUES=$(curl -s "$PROMETHEUS_URL/api/v1/label/host/values" | jq -r '.data[]?' 2>/dev/null)

if [ -z "$LABEL_VALUES" ]; then
    echo "❌ No 'host' label values found"
    echo "   The metric relabeling might not be working correctly"
else
    echo "✅ Found host label values:"
    echo "$LABEL_VALUES"
fi

echo

# Test 5: Alternative query methods for debugging
echo "5. Testing alternative queries..."

echo "All instance values:"
curl -s "$PROMETHEUS_URL/api/v1/label/instance/values" | jq -r '.data[]?' 2>/dev/null | head -3

echo

echo "Sample metric with all labels:"
curl -s "$PROMETHEUS_URL/api/v1/query?query=network_connections_info" | jq -r '.data.result[0].metric // empty' 2>/dev/null

echo

echo "=== Troubleshooting Tips ==="
echo "If host dropdown is still empty in Grafana:"
echo "1. In Grafana, go to Dashboard Settings > Variables > Host > Edit"
echo "2. Click 'Refresh' next to the query field"
echo "3. Check 'Preview of values' shows your hosts"
echo "4. If still empty, try changing the query to: label_values(host)"
echo "5. Save and go back to dashboard"
echo "6. Click the refresh icon next to the Host dropdown"
