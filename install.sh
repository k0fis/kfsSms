#!/bin/bash
# kfsSms install script for Linux (Debian/Raspberry Pi)
# Usage: curl -sL https://raw.githubusercontent.com/k0fis/kfsSms/main/install.sh | bash
#   or:  ./install.sh

set -e

INSTALL_DIR="/opt/kfsSms"
SERVICE_NAME="kfsSms"
REPO="k0fis/kfsSms"

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)  BINARY="kfsSms-linux-amd64" ;;
    armv7l|armhf)  BINARY="kfsSms-linux-arm" ;;
    aarch64)       BINARY="kfsSms-linux-arm" ;;  # 64-bit kernel, 32-bit binary works
    *)             echo "ERROR: Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "=== kfsSms installer ==="
echo "Architecture: $ARCH -> $BINARY"
echo "Install dir:  $INSTALL_DIR"
echo ""

# Check root
if [ "$(id -u)" -ne 0 ]; then
    echo "ERROR: Run as root (sudo ./install.sh)"
    exit 1
fi

# Get latest version
echo "Checking latest release..."
LATEST=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep tag_name | cut -d'"' -f4)
if [ -z "$LATEST" ]; then
    echo "ERROR: Cannot fetch latest release from GitHub"
    exit 1
fi
echo "Latest version: $LATEST"

# Create directory
mkdir -p "$INSTALL_DIR"

# Download binary
echo "Downloading $BINARY..."
curl -sL "https://github.com/$REPO/releases/download/$LATEST/$BINARY" -o "$INSTALL_DIR/kfsSms"
chmod +x "$INSTALL_DIR/kfsSms"

# Verify
if ! "$INSTALL_DIR/kfsSms" version 2>/dev/null; then
    echo "ERROR: Binary doesn't run on this system"
    exit 1
fi

# Create config template if not exists
if [ ! -f "$INSTALL_DIR/config.yml" ]; then
    echo "Creating config template..."
    cat > "$INSTALL_DIR/config.yml" << 'EOF'
sms:
  portName: "auto"
  baudRate: 115200
  pollIntervalMs: 5000
  outgoingPollIntervalMs: 5000
  openModem: true
  sendMaxRetries: 3
  sendRetryDelayMs: 5000

api:
  baseUrl: "https://sc.kofis.eu/kfsRealBotSmss"
  user: "walley"
  password: "CHANGE_ME"

msisdn:
  pin: "2562"

update:
  owner: ""
  repo: ""
EOF
    echo "  -> Edit $INSTALL_DIR/config.yml (set password!)"
else
    echo "  -> config.yml already exists, keeping"
fi

# Install systemd service
echo "Installing systemd service..."
cat > "/etc/systemd/system/$SERVICE_NAME.service" << EOF
[Unit]
Description=kfsSms SMS Gateway
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/kfsSms
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload

# Disable ModemManager if present
if systemctl is-active --quiet ModemManager 2>/dev/null; then
    echo "Disabling ModemManager (conflicts with AT commands)..."
    systemctl stop ModemManager
    systemctl disable ModemManager
fi

echo ""
echo "=== Installation complete ==="
echo ""
echo "Next steps:"
echo "  1. Edit config:    vi $INSTALL_DIR/config.yml"
echo "  2. Plug in modem:  check with 'ls /dev/ttyUSB* /dev/ttyACM*'"
echo "  3. Start service:  systemctl start $SERVICE_NAME"
echo "  4. Enable on boot: systemctl enable $SERVICE_NAME"
echo "  5. Watch logs:     journalctl -u $SERVICE_NAME -f"
echo ""
