#!/bin/bash
# update-undervolt.sh
# This script updates the undervolt Go binary in /usr/local/bin, to a new provided version
# making it available from anywhere in the terminal.
#
# Usage:
#   sudo ./update-undervolt.sh

set -e

# Ensure script is run as root.
if [[ $EUID -ne 0 ]]; then
  echo "This script must be run as root. Try using sudo."
  exit 1
fi

# Install binary to /usr/local/bin.
INSTALL_PATH="/usr/local/bin/undervolt-go-pro"

# Delete the existing binary if it exists.
if [ -f "$INSTALL_PATH" ]; then
  echo "Deleting existing binary at $INSTALL_PATH..."
  rm -f "$INSTALL_PATH"
fi

echo "Installing undervolt-go to ${INSTALL_PATH}..."
cp undervolt-go-pro "${INSTALL_PATH}"
chmod +x "${INSTALL_PATH}"

echo "Update complete! You can now use updated 'undervolt-go' from terminal. Try 'undervolt-go -h'."
