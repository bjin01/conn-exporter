#!/bin/bash
# install-conn-exporter.sh
# Simple script to install conn-exporter as a systemd service

set -e

# Configuration
BINARY_NAME="conn-exporter"
BINARY_PATH="./${BINARY_NAME}"
INSTALL_PATH="/usr/bin/${BINARY_NAME}"
SERVICE_NAME="conn-exporter"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

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

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

# Check if binary exists
check_binary() {
    if [ ! -f "$BINARY_PATH" ]; then
        log_error "Binary not found at $BINARY_PATH"
        log_info "Please ensure the conn-exporter binary is in the current directory"
        exit 1
    fi
    log_info "Found binary at $BINARY_PATH"
}

# Copy binary to /usr/bin
install_binary() {
    log_info "Installing binary to $INSTALL_PATH"
    
    # Stop service if it's running
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        log_info "Stopping existing service..."
        systemctl stop "$SERVICE_NAME"
    fi
    
    # Copy binary
    cp "$BINARY_PATH" "$INSTALL_PATH"
    chmod +x "$INSTALL_PATH"
    
    # Verify installation
    if [ -x "$INSTALL_PATH" ]; then
        log_info "Binary installed successfully to $INSTALL_PATH"
    else
        log_error "Failed to install binary"
        exit 1
    fi
}

# Create systemd service file
create_service() {
    log_info "Creating systemd service file at $SERVICE_FILE"
    
    cat > "$SERVICE_FILE" << EOF
[Unit]
Description=Network Connection Exporter
Documentation=https://github.com/your-org/conn-exporter
After=network.target

[Service]
Type=simple
ExecStart=$INSTALL_PATH
Restart=always
RestartSec=5
User=nobody
Group=nogroup

# Security settings
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=false
ProtectKernelModules=true
ProtectControlGroups=true
PrivateTmp=true
PrivateDevices=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
RestrictNamespaces=true
RestrictRealtime=true
RestrictSUIDSGID=true
LockPersonality=true
MemoryDenyWriteExecute=true
SystemCallFilter=@system-service
SystemCallErrorNumber=EPERM

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF

    if [ -f "$SERVICE_FILE" ]; then
        log_info "Service file created successfully"
    else
        log_error "Failed to create service file"
        exit 1
    fi
}

# Enable and start the service
enable_service() {
    log_info "Enabling and starting the service..."
    
    # Reload systemd daemon
    systemctl daemon-reload
    
    # Enable service to start on boot
    systemctl enable "$SERVICE_NAME"
    
    # Start the service
    systemctl start "$SERVICE_NAME"
    
    # Wait a moment for service to start
    sleep 2
    
    # Check if service is running
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        log_info "Service started successfully"
        
        # Show service status
        systemctl status "$SERVICE_NAME" --no-pager --lines=5
        
        # Test metrics endpoint
        log_info "Testing metrics endpoint..."
        sleep 1
        if curl -s --max-time 5 http://localhost:9100/metrics | grep -q network_connections_info; then
            log_info "✓ Metrics endpoint is responding correctly"
            log_info "✓ Installation completed successfully!"
        else
            log_warn "Service is running but metrics endpoint may not be ready yet"
            log_info "You can test it manually: curl http://localhost:9100/metrics"
        fi
    else
        log_error "Service failed to start"
        log_info "Check service logs: journalctl -u $SERVICE_NAME"
        exit 1
    fi
}

# Show usage information
show_usage() {
    echo "Usage: $0 [install|uninstall|status|help]"
    echo ""
    echo "Commands:"
    echo "  install   - Install conn-exporter binary and create systemd service (default)"
    echo "  uninstall - Remove conn-exporter binary and systemd service"
    echo "  status    - Show service status"
    echo "  help      - Show this help message"
    echo ""
    echo "Requirements:"
    echo "  - Must be run as root (use sudo)"
    echo "  - conn-exporter binary must be in current directory"
}

# Uninstall function
uninstall() {
    log_info "Uninstalling conn-exporter..."
    
    # Stop and disable service
    if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
        log_info "Stopping and disabling service..."
        systemctl stop "$SERVICE_NAME" 2>/dev/null || true
        systemctl disable "$SERVICE_NAME" 2>/dev/null || true
    fi
    
    # Remove service file
    if [ -f "$SERVICE_FILE" ]; then
        log_info "Removing service file..."
        rm -f "$SERVICE_FILE"
        systemctl daemon-reload
    fi
    
    # Remove binary
    if [ -f "$INSTALL_PATH" ]; then
        log_info "Removing binary..."
        rm -f "$INSTALL_PATH"
    fi
    
    log_info "Uninstallation completed"
}

# Show service status
show_status() {
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        log_info "Service is running"
        systemctl status "$SERVICE_NAME" --no-pager
        
        echo ""
        log_info "Testing metrics endpoint..."
        if curl -s --max-time 5 http://localhost:9100/metrics | head -5; then
            echo ""
            log_info "Metrics endpoint is accessible"
        else
            log_warn "Metrics endpoint is not accessible"
        fi
    else
        log_warn "Service is not running"
        if [ -f "$SERVICE_FILE" ]; then
            log_info "Service file exists, checking status..."
            systemctl status "$SERVICE_NAME" --no-pager || true
        else
            log_info "Service file does not exist"
        fi
    fi
}

# Main installation function
install() {
    log_info "Starting conn-exporter installation..."
    
    check_root
    check_binary
    install_binary
    create_service
    enable_service
    
    echo ""
    log_info "Installation summary:"
    log_info "- Binary installed to: $INSTALL_PATH"
    log_info "- Service file created: $SERVICE_FILE"
    log_info "- Service name: $SERVICE_NAME"
    log_info "- Metrics endpoint: http://localhost:9100/metrics"
    echo ""
    log_info "Useful commands:"
    log_info "- Check status: systemctl status $SERVICE_NAME"
    log_info "- View logs: journalctl -u $SERVICE_NAME -f"
    log_info "- Restart: systemctl restart $SERVICE_NAME"
    log_info "- Stop: systemctl stop $SERVICE_NAME"
}

# Main script logic
case "${1:-install}" in
    "install")
        install
        ;;
    "uninstall")
        check_root
        uninstall
        ;;
    "status")
        show_status
        ;;
    "help"|"-h"|"--help")
        show_usage
        ;;
    *)
        log_error "Unknown command: $1"
        show_usage
        exit 1
        ;;
esac