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

# Move to the script's directory so ./uninstall.sh and ./install.sh are found.
cd "$(dirname "$0")"

echo "Starting update..."

# Export DESTDIR so sub-scripts see it
export DESTDIR

bash ./uninstall-undervolt.sh --keepconfig --keeppersist --keepautoswitch
bash ./install-undervolt.sh

echo "Update complete! You can now use the updated application from the terminal, applications menu or desktop. Try 'sudo undervolt-go-pro'."