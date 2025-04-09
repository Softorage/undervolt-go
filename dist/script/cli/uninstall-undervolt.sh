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

# Check if the file exists before attempting to remove it
if [[ -f "${INSTALL_PATH}" ]]; then
  echo "Removing ${INSTALL_PATH}..."
  rm -f "${INSTALL_PATH}"
  echo "Uninstallation complete. 'undervolt-go' has been removed."
else
  echo "No installation found at ${INSTALL_PATH}."
fi