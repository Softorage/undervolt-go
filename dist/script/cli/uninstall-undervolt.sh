#!/bin/bash
# uninstall-undervolt.sh
# This script removes the undervolt Go binary from /usr/local/bin.
#
# Usage:
#   sudo ./uninstall-undervolt.sh

set -e

# Ensure script is run as root.
if [[ $EUID -ne 0 ]]; then
  echo "This script must be run as root. Try using sudo."
  exit 1
fi

INSTALL_PATH="/usr/local/bin/undervolt-go"

# New paths created by the Go program
PERSIST_SERVICE="/etc/systemd/system/undervolt-go.service"
AUTO_SERVICE="/etc/systemd/system/undervolt-go-auto.service"
AUTO_UDEV="/etc/udev/rules.d/99-undervolt-go-auto.rules"
CONFIG_DIR="/etc/undervolt-go"

# Stop and disable systemd services if they are running
echo "Stopping and disabling systemd services created by undervolt-go..."
systemctl stop undervolt-go.service undervolt-go-auto.service 2>/dev/null || true
systemctl disable undervolt-go.service undervolt-go-auto.service 2>/dev/null || true

# Remove persistence systemd service
if [[ -f "${PERSIST_SERVICE}" ]]; then
  echo "Removing persistence service at ${PERSIST_SERVICE}..."
  rm -f "${PERSIST_SERVICE}"
fi

# Remove auto-switch systemd service
if [[ -f "${AUTO_SERVICE}" ]]; then
  echo "Removing auto-switch service at ${AUTO_SERVICE}..."
  rm -f "${AUTO_SERVICE}"
fi

# Remove auto-switch udev rule
if [[ -f "${AUTO_UDEV}" ]]; then
  echo "Removing udev rule at ${AUTO_UDEV}..."
  rm -f "${AUTO_UDEV}"
fi

# Remove configuration directory and config.yaml
if [[ -d "${CONFIG_DIR}" ]]; then
  echo "Removing configuration directory at ${CONFIG_DIR}..."
  rm -rf "${CONFIG_DIR}"
fi

# Reload systemd and udev daemons to reflect changes
echo "Reloading systemd and udev..."
systemctl daemon-reload
systemctl reset-failed
udevadm control --reload-rules

# Check if the file exists before attempting to remove it
if [[ -f "${INSTALL_PATH}" ]]; then
  echo "Removing ${INSTALL_PATH}..."
  rm -f "${INSTALL_PATH}"
  echo "Uninstallation complete. 'undervolt-go' has been removed."
else
  echo "No installation found at ${INSTALL_PATH}."
fi