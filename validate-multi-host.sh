#!/bin/bash

# Multi-Host conn-exporter Validation Script
# This script validates that conn-exporter is working correctly on all target servers

set -e

# Configuration
SERVERS=("sap03.susedemo.de" "bokvm.susedemo.de")
PORT="9100"
TIMEOUT="10"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=== Multi-Host conn-exporter Validation ==="
echo "Testing servers: ${SERVERS[@]}"
echo "Port: $PORT"
echo "Timeout: $TIMEOUT seconds"
echo ""

# Function to test a single server
test_server() {
    local server=$1
    local url="http://${server}:${PORT}/metrics"
    
    echo -n "Testing $server... "
    
    # Test connectivity
    if ! curl -s --connect-timeout $TIMEOUT "$url" >/dev/null 2>&1; then
        echo -e "${RED}FAILED${NC} - Cannot connect to $url"
        return 1
    fi
    
    # Get metrics
    local metrics=$(curl -s --connect-timeout $TIMEOUT "$url")
    
    # Check if conn-exporter metrics are present
    local conn_metrics=$(echo "$metrics" | grep -c "network_connections_info" || true)
    if [ "$conn_metrics" -eq 0 ]; then
        echo -e "${RED}FAILED${NC} - No network_connections_info metrics found"
        return 1
    fi
    
    # Check for unknown interfaces
    local unknown_interfaces=$(echo "$metrics" | grep -c 'interface="unknown"' || true)
    
    # Get interface distribution
    local interfaces=$(echo "$metrics" | grep "network_connections_info" | \
                     sed -n 's/.*interface="\([^"]*\)".*/\1/p' | sort | uniq -c | \
                     awk '{printf "%s:%s ", $2, $1}')
    
    echo -e "${GREEN}OK${NC} - $conn_metrics connections found"
    echo "  Interfaces: $interfaces"
    
    if [ "$unknown_interfaces" -gt 0 ]; then
        echo -e "  ${YELLOW}WARNING${NC}: $unknown_interfaces connections with unknown interfaces"
    fi
    
    return 0
}

# Function to test Prometheus connectivity
test_prometheus_config() {
    echo ""
    echo "=== Prometheus Configuration Validation ==="
    
    # Check if prometheus.yml mentions our servers
    if [ -f "/etc/prometheus/prometheus.yml" ]; then
        echo "Checking Prometheus configuration..."
        
        for server in "${SERVERS[@]}"; do
            if grep -q "$server" /etc/prometheus/prometheus.yml; then
                echo -e "  ${GREEN}✓${NC} $server found in prometheus.yml"
            else
                echo -e "  ${YELLOW}!${NC} $server NOT found in prometheus.yml"
            fi
        done
    else
        echo -e "${YELLOW}WARNING${NC}: /etc/prometheus/prometheus.yml not found"
        echo "Make sure to add these targets to your Prometheus configuration:"
        for server in "${SERVERS[@]}"; do
            echo "  - '$server:$PORT'"
        done
    fi
}

# Function to generate sample queries
generate_sample_queries() {
    echo ""
    echo "=== Sample PromQL Queries ==="
    cat << EOF
# Total connections across all servers:
sum(network_connections_info{job="conn-exporter-multi-host"})

# Connections per server:
sum(network_connections_info{job="conn-exporter-multi-host"}) by (instance)

# Check server health:
up{job="conn-exporter-multi-host"}

# SSH connections across all servers:
sum(network_connections_info{job="conn-exporter-multi-host", source_port="22"}) by (instance)

# Network interface utilization:
sum(network_connections_info{job="conn-exporter-multi-host"}) by (instance, interface)
EOF
}

# Main execution
echo "Starting validation..."
echo ""

failed_servers=0
total_connections=0

for server in "${SERVERS[@]}"; do
    if test_server "$server"; then
        # Count total connections for this server
        local_connections=$(curl -s "http://${server}:${PORT}/metrics" | \
                          grep -c "network_connections_info" || true)
        total_connections=$((total_connections + local_connections))
    else
        failed_servers=$((failed_servers + 1))
    fi
    echo ""
done

# Summary
echo "=== Summary ==="
echo "Servers tested: ${#SERVERS[@]}"
echo "Servers successful: $((${#SERVERS[@]} - failed_servers))"
echo "Servers failed: $failed_servers"
echo "Total connections monitored: $total_connections"

if [ $failed_servers -eq 0 ]; then
    echo -e "${GREEN}✓ All servers are working correctly!${NC}"
else
    echo -e "${RED}✗ $failed_servers server(s) have issues${NC}"
fi

# Additional checks
test_prometheus_config
generate_sample_queries

echo ""
echo "=== Next Steps ==="
echo "1. Add the working servers to your Prometheus prometheus.yml"
echo "2. Configure Grafana dashboards using the sample queries above"
echo "3. Set up alerting rules from conn-exporter-alerts.yml"
echo "4. Monitor logs: sudo journalctl -u conn-exporter -f"

exit $failed_servers
