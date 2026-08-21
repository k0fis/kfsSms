#!/bin/bash
# kfsSms update script — check for new version and restart if updated
# Put in cron: 0 4 * * * /opt/kfsSms/update.sh

set -e

INSTALL_DIR="/opt/kfsSms"
REPO="k0fis/kfsSms"
SERVICE_NAME="kfsSms"

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)  BINARY="kfsSms-linux-amd64" ;;
    armv7l|armhf)  BINARY="kfsSms-linux-arm" ;;
    aarch64)       BINARY="kfsSms-linux-arm" ;;
    *)             echo "Unsupported: $ARCH"; exit 1 ;;
esac

CURRENT=$("$INSTALL_DIR/kfsSms" version 2>/dev/null | awk '{print $2}')
LATEST=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep tag_name | cut -d'"' -f4)

if [ -z "$LATEST" ]; then
    echo "$(date): Cannot check updates" >> /var/log/kfsSms-update.log
    exit 1
fi

if [ "$CURRENT" = "$LATEST" ]; then
    exit 0
fi

echo "$(date): Updating $CURRENT -> $LATEST" >> /var/log/kfsSms-update.log

curl -sL "https://github.com/$REPO/releases/download/$LATEST/$BINARY" -o "$INSTALL_DIR/kfsSms-new"
chmod +x "$INSTALL_DIR/kfsSms-new"

# Verify new binary runs
if "$INSTALL_DIR/kfsSms-new" version >/dev/null 2>&1; then
    mv "$INSTALL_DIR/kfsSms-new" "$INSTALL_DIR/kfsSms"
    systemctl restart "$SERVICE_NAME"
    echo "$(date): Updated to $LATEST, service restarted" >> /var/log/kfsSms-update.log
else
    rm -f "$INSTALL_DIR/kfsSms-new"
    echo "$(date): New binary failed verification" >> /var/log/kfsSms-update.log
    exit 1
fi
