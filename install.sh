#!/bin/bash

set -e

REPO="tocha688/go-visitor-web"
SERVICE_NAME="visitor"
INSTALL_DIR="/opt/visitor"
BIN_DIR="/usr/local/bin"
CONFIG_FILE="$INSTALL_DIR/config.yaml"
SERVICE_FILE="/etc/systemd/system/$SERVICE_NAME.service"

echo "=== Visitor Tracker Installer ==="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root: sudo $0"
    exit 1
fi

# Get latest release version
echo "Fetching latest release..."
LATEST_VERSION=$(curl -s https://api.github.com/repos/$REPO/releases/latest | grep -oP '"tag_name":\s*"\K[^"]+' | sed 's/v//')
if [ -z "$LATEST_VERSION" ]; then
    echo "Failed to fetch latest version"
    exit 1
fi
echo "Latest version: v$LATEST_VERSION"

# Detect architecture
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
echo "Detected architecture: $ARCH ($ARCH_NAME)"

# Download URL
FILENAME="visitor-linux-$ARCH_NAME.tar.gz"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/v$LATEST_VERSION/$FILENAME"

echo ""
echo "Downloading $FILENAME..."
cd /tmp
curl -# -L -o "$FILENAME" "$DOWNLOAD_URL"

# Extract
echo "Extracting..."
tar -xzf "$FILENAME"

# Install binary
echo "Installing..."
mkdir -p "$INSTALL_DIR"
cp visitor-$ARCH_NAME "$INSTALL_DIR/$SERVICE_NAME" 2>/dev/null || cp visitor-linux-$ARCH_NAME "$INSTALL_DIR/$SERVICE_NAME"
chmod +x "$INSTALL_DIR/$SERVICE_NAME"

# Create default config if not exists
if [ ! -f "$CONFIG_FILE" ]; then
    cat > "$CONFIG_FILE" << 'EOF'
app:
  host: "0.0.0.0"
  port: 8080
  admin_password: "123456"
  target_url: "https://www.example.com"
stats:
  visit_file: "data/visits.json"
EOF
    echo "Config created at $CONFIG_FILE"
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
ExecStart=$INSTALL_DIR/$SERVICE_NAME
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd and enable service
echo "Enabling service..."
systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
systemctl start "$SERVICE_NAME"

# Cleanup
rm -f /tmp/$FILENAME
rm -f /tmp/visitor-$ARCH_NAME

echo ""
echo "=== Installation Complete ==="
echo "Service: $SERVICE_NAME"
echo "Config: $CONFIG_FILE"
echo ""
systemctl status "$SERVICE_NAME" --no-pager
echo ""
echo "View logs: journalctl -u $SERVICE_NAME -f"
