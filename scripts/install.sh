#!/bin/bash

set -e

REPO="tocha688/go-visitor-web"
SERVICE_NAME="visitor"
INSTALL_DIR="/opt/visitor"
BIN_NAME="visitor"
CLI_NAME="vtor"
CONFIG_FILE="$INSTALL_DIR/config.yaml"
SERVICE_FILE="/etc/systemd/system/$SERVICE_NAME.service"
CLI_FILE="/usr/local/bin/$CLI_NAME"

# Default values
PORT="8080"
MODE="local"
REMOVE=false

usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -w, --web      Install from GitHub (download latest release)"
    echo "  -p, --port     Set port (default: 8080)"
    echo "  -r, --remove   Uninstall service and files"
    echo "  -h, --help     Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                      # Local install using files in current directory"
    echo "  $0 -w                    # Download and install latest release from GitHub"
    echo "  $0 -p 9000              # Install with custom port"
    echo "  $0 -r                   # Uninstall"
    exit 1
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -w|--web)
            MODE="web"
            shift
            ;;
        -p|--port)
            PORT="$2"
            shift 2
            ;;
        -r|--remove)
            REMOVE=true
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

# Check root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        echo "Please run as root: sudo $0"
        exit 1
    fi
}

# Uninstall
uninstall() {
    check_root
    echo "=== Uninstalling Visitor Tracker ==="
    
    # Stop and disable service
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        echo "Stopping service..."
        systemctl stop "$SERVICE_NAME"
    fi
    if systemctl is-enabled --quiet "$SERVICE_NAME"; then
        echo "Disabling service..."
        systemctl disable "$SERVICE_NAME"
    fi
    
    # Remove service file
    if [ -f "$SERVICE_FILE" ]; then
        echo "Removing service file..."
        rm -f "$SERVICE_FILE"
        systemctl daemon-reload
    fi
    
    # Remove CLI
    if [ -f "$CLI_FILE" ]; then
        echo "Removing CLI..."
        rm -f "$CLI_FILE"
    fi
    
    # Remove installation directory
    if [ -d "$INSTALL_DIR" ]; then
        echo "Removing installation directory..."
        rm -rf "$INSTALL_DIR"
    fi
    
    echo "=== Uninstall Complete ==="
    exit 0
}

# Get latest version from GitHub
get_latest_version() {
    LATEST_VERSION=$(curl -s https://api.github.com/repos/$REPO/releases/latest | grep -oP '"tag_name":\s*"\K[^"]+' | sed 's/v//')
    if [ -z "$LATEST_VERSION" ]; then
        echo "Failed to fetch latest version"
        exit 1
    fi
}

# Detect architecture
detect_arch() {
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            ARCH_NAME="amd64"
            ;;
        i386|i686)
            ARCH_NAME="386"
            ;;
        aarch64|arm64)
            ARCH_NAME="arm64"
            ;;
        *)
            echo "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac
}

# Download from GitHub
download_from_web() {
    check_root
    echo "=== Installing Visitor Tracker from GitHub ==="
    echo ""
    
    get_latest_version
    detect_arch
    echo "Latest version: v$LATEST_VERSION"
    echo "Architecture: $ARCH_NAME"
    echo ""
    
    FILENAME="visitor-linux-$ARCH_NAME.tar.gz"
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/v$LATEST_VERSION/$FILENAME"
    
    echo "Downloading $FILENAME..."
    cd /tmp
    curl -# -L -o "$FILENAME" "$DOWNLOAD_URL"
    
    echo "Extracting..."
    tar -xzf "$FILENAME"
    
    install_files
}

# Install from local files
install_local() {
    check_root
    echo "=== Installing Visitor Tracker from local files ==="
    echo ""
    
    if [ ! -f "$BIN_NAME" ]; then
        echo "Error: Binary '$BIN_NAME' not found in current directory"
        exit 1
    fi
    
    install_files
}

# Install files
install_files() {
    echo "Installing..."
    mkdir -p "$INSTALL_DIR"
    
    # Install binary
    cp "$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
    chmod +x "$INSTALL_DIR/$BIN_NAME"
    
    # Install CLI
    if [ -f "$CLI_NAME" ]; then
        cp "$CLI_NAME" "$CLI_FILE"
        chmod +x "$CLI_FILE"
        echo "CLI installed to $CLI_FILE"
    fi
    
    # Create config if not exists
    if [ ! -f "$CONFIG_FILE" ]; then
        cat > "$CONFIG_FILE" << EOF
app:
  host: "0.0.0.0"
  port: $PORT
  admin_password: "123456"
  target_url: "https://www.example.com"
stats:
  visit_file: "data/visits.json"
EOF
        echo "Config created at $CONFIG_FILE"
    else
        # Update port in existing config
        sed -i "s/port:.*/port: $PORT/" "$CONFIG_FILE"
        echo "Port updated to $PORT"
    fi
    
    # Create data directory
    mkdir -p "$INSTALL_DIR/data"
    
    # Create systemd service
    echo "Creating systemd service..."
    cat > "$SERVICE_FILE" << EOF
[Unit]
Description=Visitor Tracker Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/$BIN_NAME
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
    
    # Reload systemd and enable service
    echo "Enabling service..."
    systemctl daemon-reload
    systemctl enable "$SERVICE_NAME"
    systemctl restart "$SERVICE_NAME"
    
    # Cleanup
    rm -f /tmp/$FILENAME 2>/dev/null
    rm -f /tmp/$BIN_NAME 2>/dev/null
    rm -f /tmp/$CLI_NAME 2>/dev/null
    
    echo ""
    echo "=== Installation Complete ==="
    echo "Service: $SERVICE_NAME"
    echo "Config: $CONFIG_FILE"
    echo "CLI: $CLI_FILE"
    echo "Port: $PORT"
    echo ""
    systemctl status "$SERVICE_NAME" --no-pager
    echo ""
    echo "Usage:"
    echo "  visitor start|stop|restart|status  - Manage service"
    echo "  visitor port <num>               - Change port"
    echo "  visitor password <pwd>          - Change password"
    echo "  visitor log                     - View logs"
    echo "  visitor config                  - Show config"
}

# Main
if [ "$REMOVE" = true ]; then
    uninstall
elif [ "$MODE" = "web" ]; then
    download_from_web
else
    install_local
fi
