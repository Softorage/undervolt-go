#!/bin/bash
# uninstall-undervolt.sh
# Uninstalls undervolt-go-pro. Supports flags to retain certain configurations.

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

# Ensure script is run as root
if [[ $EUID -ne 0 ]]; then
  echo "This script must be run as root. Try using sudo."
  exit 1
fi

echo "Uninstalling Undervolt Go Pro..."

# Define paths
INSTALL_PATH="/usr/local/bin/undervolt-go-pro"
WRAPPER_PATH="/usr/bin/undervolt-go-wrapper"
OLD_ICON_PATH="/usr/share/icons/undervolt-go.png"
ICON_PATH="/usr/share/pixmaps/undervolt-go.png"
POLKIT_FILE="/usr/share/polkit-1/actions/com.softorage.undervolt-go.policy"
DESKTOP_FILE="/usr/share/applications/undervolt-go.desktop"

# Paths created by the Go program
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
else
  echo "No installation found at ${INSTALL_PATH}."
fi

# Remove wrapper
if [[ -f "${WRAPPER_PATH}" ]]; then
  echo "Removing wrapper at ${WRAPPER_PATH}..."
  rm -f "${WRAPPER_PATH}"
fi

# Remove icon
if [[ -f "${ICON_PATH}" ]]; then
  echo "Removing icon at ${ICON_PATH}..."
  rm -f "${ICON_PATH}"
fi

# Remove icon at old path
if [[ -f "${OLD_ICON_PATH}" ]]; then
  rm -f "${OLD_ICON_PATH}"
fi

# Remove PolicyKit file
if [[ -f "${POLKIT_FILE}" ]]; then
  echo "Removing PolicyKit file at ${POLKIT_FILE}..."
  rm -f "${POLKIT_FILE}"
fi

# Remove desktop entry
if [[ -f "${DESKTOP_FILE}" ]]; then
  echo "Removing desktop entry at ${DESKTOP_FILE}..."
  rm -f "${DESKTOP_FILE}"
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

# Remove desktop shortcut from user's Desktop
USER_DESKTOP="${SUDO_USER:-$USER}"
USER_HOME=$(eval echo "~${USER_DESKTOP}")
USER_DESKTOP_FILE="${USER_HOME}/Desktop/undervolt-go.desktop"
if [[ -f "${USER_DESKTOP_FILE}" ]]; then
  echo "Removing desktop shortcut at ${USER_DESKTOP_FILE}..."
  rm -f "${USER_DESKTOP_FILE}"
fi

echo "Uninstallation complete."