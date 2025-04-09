#!/bin/bash
# install-undervolt.sh
# This script installs the undervolt Go binary to /usr/local/bin,
# making it available from anywhere in the terminal.
#
# Usage:
#   sudo ./install-undervolt.sh

set -e

# Ensure script is run as root.
if [[ $EUID -ne 0 ]]; then
  echo "This script must be run as root. Try using sudo."
  exit 1
fi

# Install binary to /usr/local/bin.
INSTALL_PATH="/usr/local/bin/undervolt-go"
echo "Installing undervolt-go to ${INSTALL_PATH}..."
cp undervolt-go "${INSTALL_PATH}"
chmod +x "${INSTALL_PATH}"

echo "Installation complete! You can now use 'undervolt-go' from terminal. Try 'undervolt-go -h'."
