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

# Define paths
INSTALL_PATH="/usr/local/bin/undervolt-go-pro"
ICON_PATH="/usr/share/icons/undervolt-go.png"

# Delete the existing binary if it exists.
if [ -f "$INSTALL_PATH" ]; then
  echo "Deleting existing binary at $INSTALL_PATH..."
  rm -f "$INSTALL_PATH"
fi

# Install the new binary
echo "Installing undervolt-go-pro to ${INSTALL_PATH}..."
cp undervolt-go-pro "${INSTALL_PATH}"
chmod +x "${INSTALL_PATH}"

# Update icon if present in the current directory
if [[ -f "icon.png" ]]; then
  # Delete the existing icon if it exists
  if [ -f "$ICON_PATH" ]; then
    echo "Deleting existing icon at $ICON_PATH..."
    rm -f "$ICON_PATH"
  fi
  # Copy the new icon
  echo "Installing new icon to ${ICON_PATH}..."
  cp icon.png "${ICON_PATH}"
  chmod 644 "${ICON_PATH}"
else
  echo "No icon.png found in the current directory. Skipping icon update."
fi

echo "Update complete! You can now use the updated application from the terminal, applications menu or desktop. Try 'sudo undervolt-go-pro'."