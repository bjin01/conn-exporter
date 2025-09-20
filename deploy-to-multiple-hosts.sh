#!/bin/bash
# deploy-to-multiple-hosts.sh
# Script to deploy conn-exporter to multiple hosts

set -e

# Configuration
BINARY_PATH="./conn-exporter"
SERVICE_PORT="9100"

# Host definitions - modify these for your environment
declare -A HOSTS=(
    ["web-server-01"]="user@192.168.1.10"
    ["web-server-02"]="user@192.168.1.11" 
    ["db-server-01"]="user@192.168.1.20"
    ["app-server-01"]="user@10.0.1.10"
    ["dev-server-01"]="user@192.168.2.10"
)

# Environment labels for each host
declare -A ENVIRONMENTS=(
    ["web-server-01"]="production"
    ["web-server-02"]="production"
    ["db-server-01"]="production"
    ["app-server-01"]="production"
    ["dev-server-01"]="development"
)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if binary exists
check_binary() {
    if [ ! -f "$BINARY_PATH" ]; then
        log_error "Binary not found at $BINARY_PATH"
        log_info "Please build the binary first: go build -o conn-exporter ."
        exit 1
    fi
    log_info "Binary found at $BINARY_PATH"
}

# Deploy to a single host
deploy_to_host() {
    local host_name=$1
    local ssh_target=$2
    local environment=${ENVIRONMENTS[$host_name]}
    
    log_info "Deploying to $host_name ($ssh_target) [Environment: $environment]"
    
    # Copy binary
    log_info "Copying binary to $host_name..."
    if ! scp "$BINARY_PATH" "$ssh_target:/tmp/conn-exporter"; then
        log_error "Failed to copy binary to $host_name"
        return 1
    fi
    
    # Setup and start service
    log_info "Setting up service on $host_name..."
    if ! ssh "$ssh_target" 'bash -s' << 'EOF'
        set -e
        
        # Create user if not exists
        if ! id conn-exporter &>/dev/null; then
            sudo useradd --system --shell /bin/false --home /opt/conn-exporter --create-home conn-exporter
            echo "Created conn-exporter user"
        fi
        
        # Setup directories and binary
        sudo mkdir -p /opt/conn-exporter
        sudo mv /tmp/conn-exporter /opt/conn-exporter/
        sudo chown -R conn-exporter:conn-exporter /opt/conn-exporter
        sudo chmod +x /opt/conn-exporter/conn-exporter
        
        # Create systemd service
        sudo tee /etc/systemd/system/conn-exporter.service > /dev/null << 'EOL'
[Unit]
Description=Network Connection Exporter
Documentation=https://github.com/your-org/conn-exporter
After=network.target

[Service]
Type=simple
User=conn-exporter
Group=conn-exporter
ExecStart=/opt/conn-exporter/conn-exporter
Restart=always
RestartSec=5
LimitNOFILE=65536

# Security settings
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/conn-exporter

[Install]
WantedBy=multi-user.target
EOL
        
        # Reload systemd and start service
        sudo systemctl daemon-reload
        sudo systemctl enable conn-exporter
        
        # Stop if running and start
        sudo systemctl stop conn-exporter 2>/dev/null || true
        sudo systemctl start conn-exporter
        
        # Wait a moment for service to start
        sleep 2
        
        # Check if service is running
        if sudo systemctl is-active --quiet conn-exporter; then
            echo "Service started successfully"
            
            # Test metrics endpoint
            if curl -s http://localhost:9100/metrics | grep -q network_connections_info; then
                echo "Metrics endpoint is responding correctly"
            else
                echo "WARNING: Metrics endpoint not responding properly"
                exit 1
            fi
        else
            echo "ERROR: Service failed to start"
            sudo systemctl status conn-exporter --no-pager
            exit 1
        fi
EOF
    then
        log_error "Failed to setup service on $host_name"
        return 1
    fi
    
    log_info "Successfully deployed to $host_name"
    return 0
}

# Generate Prometheus configuration
generate_prometheus_config() {
    local config_file="prometheus-targets-generated.yml"
    
    log_info "Generating Prometheus configuration..."
    
    cat > "$config_file" << 'EOF'
# Generated Prometheus configuration for conn-exporter multi-host setup
# Add this to your prometheus.yml file under scrape_configs:

scrape_configs:
  - job_name: 'conn-exporter-multi-host'
    scrape_interval: 15s
    scrape_timeout: 10s
    metrics_path: /metrics
    scheme: http
    
    static_configs:
EOF

    # Group hosts by environment
    declare -A env_hosts
    for host in "${!HOSTS[@]}"; do
        env="${ENVIRONMENTS[$host]}"
        if [[ -z "${env_hosts[$env]}" ]]; then
            env_hosts[$env]="$host"
        else
            env_hosts[$env]="${env_hosts[$env]} $host"
        fi
    done
    
    # Generate config for each environment
    for env in "${!env_hosts[@]}"; do
        cat >> "$config_file" << EOF
      # $env environment
      - targets:
EOF
        
        for host in ${env_hosts[$env]}; do
            ssh_target="${HOSTS[$host]}"
            # Extract IP/hostname from user@host format
            target_host="${ssh_target#*@}"
            cat >> "$config_file" << EOF
          - '$target_host:$SERVICE_PORT'
EOF
        done
        
        cat >> "$config_file" << EOF
        labels:
          environment: '$env'
          datacenter: 'dc1'  # Modify as needed
          
EOF
    done
    
    # Add relabeling config
    cat >> "$config_file" << 'EOF'
    relabel_configs:
      # Extract hostname from target address
      - source_labels: [__address__]
        target_label: __tmp_host
        regex: '([^:]+)(:\d+)?'
        replacement: '${1}'
      
      # Create short hostname label
      - source_labels: [__tmp_host]
        target_label: host
        regex: '([^.]+)\..*'
        replacement: '${1}'
      
      # Keep full hostname
      - source_labels: [__tmp_host]
        target_label: hostname
        regex: '(.+)'
        replacement: '${1}'
EOF
    
    log_info "Prometheus configuration saved to $config_file"
}

# Test all deployed instances
test_deployments() {
    log_info "Testing all deployments..."
    
    local failed_hosts=()
    
    for host_name in "${!HOSTS[@]}"; do
        ssh_target="${HOSTS[$host_name]}"
        target_host="${ssh_target#*@}"
        
        log_info "Testing $host_name ($target_host)..."
        
        if curl -s --max-time 5 "http://$target_host:$SERVICE_PORT/metrics" | grep -q network_connections_info; then
            log_info "✓ $host_name is working correctly"
        else
            log_error "✗ $host_name is not responding properly"
            failed_hosts+=("$host_name")
        fi
    done
    
    if [ ${#failed_hosts[@]} -eq 0 ]; then
        log_info "All deployments are working correctly!"
    else
        log_error "Failed hosts: ${failed_hosts[*]}"
        return 1
    fi
}

# Main execution
main() {
    log_info "Starting conn-exporter multi-host deployment"
    
    # Check binary
    check_binary
    
    # Deploy to all hosts
    local failed_deployments=()
    
    for host_name in "${!HOSTS[@]}"; do
        if ! deploy_to_host "$host_name" "${HOSTS[$host_name]}"; then
            failed_deployments+=("$host_name")
        fi
        echo "---"
    done
    
    # Report results
    if [ ${#failed_deployments[@]} -eq 0 ]; then
        log_info "All deployments completed successfully!"
    else
        log_error "Failed deployments: ${failed_deployments[*]}"
    fi
    
    # Generate Prometheus config
    generate_prometheus_config
    
    # Test deployments
    echo "---"
    test_deployments
    
    log_info "Deployment complete!"
    log_info "Next steps:"
    log_info "1. Add the generated Prometheus configuration to your prometheus.yml"
    log_info "2. Restart Prometheus"
    log_info "3. Import the Grafana dashboard: grafana-dashboard-multi-host.json"
}

# Handle script arguments
case "${1:-deploy}" in
    "deploy")
        main
        ;;
    "test")
        test_deployments
        ;;
    "config")
        generate_prometheus_config
        ;;
    "help"|"-h"|"--help")
        echo "Usage: $0 [deploy|test|config|help]"
        echo "  deploy: Deploy conn-exporter to all configured hosts (default)"
        echo "  test:   Test all deployed instances"
        echo "  config: Generate Prometheus configuration only"
        echo "  help:   Show this help message"
        ;;
    *)
        log_error "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac