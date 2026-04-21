#!/bin/bash
# uninstall-undervolt.sh
# This script removes the undervolt Go binary from /usr/local/bin.
#
# Usage:
#   sudo ./uninstall-undervolt.sh

set -e

# Default flag states
KEEP_CONFIG=false
KEEP_PERSIST=false
KEEP_AUTOSWITCH=false

# Parse command-line arguments
while [[ "$#" -gt 0 ]]; do
  case $1 in
    --keepconfig) KEEP_CONFIG=true ;;
    --keeppersist) KEEP_PERSIST=true ;;
    --keepautoswitch) KEEP_AUTOSWITCH=true ;;
    -h|--help)
      echo "Usage: $0 [options]"
      echo "Options:"
      echo "  --keepconfig      Keep the configuration directory and files."
      echo "  --keeppersist     Keep the systemd service for persistence."
      echo "  --keepautoswitch  Keep the udev rule and systemd service for auto-switching."
      echo "  -h, --help       Show this help message."
      exit 0
      ;;
    *)
      echo "Unknown parameter passed: $1"
      echo "Use -h or --help for usage."
      exit 1
      ;;
  esac
  shift
done

# Ensure script is run as root.
if [[ $EUID -ne 0 ]]; then
  echo "This script must be run as root. Try using sudo."
  exit 1
fi

echo "Uninstalling Undervolt Go..."

INSTALL_PATH="/usr/local/bin/undervolt-go"

# New paths created by the Go program
PERSIST_SERVICE="/etc/systemd/system/undervolt-go.service"
AUTO_SERVICE="/etc/systemd/system/undervolt-go-auto.service"
AUTO_UDEV="/etc/udev/rules.d/99-undervolt-go-auto.rules"
CONFIG_DIR="/etc/undervolt-go"

# Stop systemd services if they are running, as the binaries are being removed.
echo "Stopping systemd services..."
systemctl stop undervolt-go.service undervolt-go-auto.service 2>/dev/null || true

# Remove binary
if [[ -f "${INSTALL_PATH}" ]]; then
  echo "Removing binary at ${INSTALL_PATH}..."
  rm -f "${INSTALL_PATH}"
fi

# Persistence systemd service
if [[ "$KEEP_PERSIST" = false ]]; then
  systemctl disable undervolt-go.service 2>/dev/null || true
  if [[ -f "${PERSIST_SERVICE}" ]]; then
    echo "Removing persistence service at ${PERSIST_SERVICE}..."
    rm -f "${PERSIST_SERVICE}"
  fi
else
  echo "Keeping persistence service at ${PERSIST_SERVICE}..."
fi

# Auto-switch systemd service and udev rule
if [[ "$KEEP_AUTOSWITCH" = false ]]; then
  systemctl disable undervolt-go-auto.service 2>/dev/null || true
  if [[ -f "${AUTO_SERVICE}" ]]; then
    echo "Removing auto-switch service at ${AUTO_SERVICE}..."
    rm -f "${AUTO_SERVICE}"
  fi
  if [[ -f "${AUTO_UDEV}" ]]; then
    echo "Removing udev rule at ${AUTO_UDEV}..."
    rm -f "${AUTO_UDEV}"
  fi
else
  echo "Keeping auto-switch service and udev rules..."
fi

# Configuration directory and config.yaml
if [[ "$KEEP_CONFIG" = false ]]; then
  if [[ -d "${CONFIG_DIR}" ]]; then
    echo "Removing configuration directory at ${CONFIG_DIR}..."
    rm -rf "${CONFIG_DIR}"
  fi
else
  echo "Keeping configuration directory at ${CONFIG_DIR}..."
fi

# Reload systemd and udev daemons to reflect changes
echo "Reloading systemd and udev..."
systemctl daemon-reload
systemctl reset-failed
udevadm control --reload-rules

echo "Uninstallation complete."
