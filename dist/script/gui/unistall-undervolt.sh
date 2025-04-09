#!/bin/bash
# uninstall-undervolt.sh
# Uninstalls undervolt-go-pro and removes all associated files.

set -e

# Ensure script is run as root
if [[ $EUID -ne 0 ]]; then
  echo "This script must be run as root. Try using sudo."
  exit 1
fi

echo "Uninstalling Undervolt Go..."

# Define paths
INSTALL_PATH="/usr/local/bin/undervolt-go-pro"
WRAPPER_PATH="/usr/bin/undervolt-go-wrapper"
ICON_PATH="/usr/share/icons/undervolt-go.png"
POLKIT_FILE="/usr/share/polkit-1/actions/com.softorage.undervolt-go.policy"
DESKTOP_FILE="/usr/share/applications/undervolt-go.desktop"

# Remove binary
if [[ -f "${INSTALL_PATH}" ]]; then
  echo "Removing binary at ${INSTALL_PATH}..."
  rm -f "${INSTALL_PATH}"
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

# Remove desktop shortcut from user's Desktop
USER_DESKTOP="${SUDO_USER:-$USER}"
USER_HOME=$(eval echo "~${USER_DESKTOP}")
USER_DESKTOP_FILE="${USER_HOME}/Desktop/undervolt-go.desktop"
if [[ -f "${USER_DESKTOP_FILE}" ]]; then
  echo "Removing desktop shortcut at ${USER_DESKTOP_FILE}..."
  rm -f "${USER_DESKTOP_FILE}"
fi

echo "Uninstallation complete. All files have been removed."